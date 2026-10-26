package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/apikey"
	appViewer "go-wind-ops/pkg/entgo/viewer"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ApiKeyRepo OpenAPI 密钥仓储(手写转换,同 pipeline 范式)。
type ApiKeyRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewApiKeyRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ApiKeyRepo {
	return &ApiKeyRepo{
		log:       ctx.NewLoggerHelper("api-key/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO 实体 → proto(不含明文/哈希)。
func (r *ApiKeyRepo) toDTO(e *ent.ApiKey) *opsV1.ApiKey {
	dto := &opsV1.ApiKey{
		Id:          &e.ID,
		Name:        &e.Name,
		Prefix:      &e.Prefix,
		Description: e.Description,
		CreatedAt:   timeTimestamppb(e.CreatedAt),
		LastUsedAt:  timeTimestamppb(e.LastUsedAt),
		ExpiresAt:   timeTimestamppb(e.ExpiresAt),
	}
	// 归属信息:OpenAPI 网关据此注入操作者/租户归因(触发的构建/分发/流水线
	// 落到密钥创建者的租户,而非平台)。
	if e.CreatedBy != nil {
		dto.OwnerUserId = trans.Ptr(uint32(*e.CreatedBy))
	}
	if e.TenantID != nil {
		dto.TenantId = e.TenantID
	}
	if e.Status != nil {
		status := opsV1.ApiKey_ENABLED
		if *e.Status == apikey.StatusDisabled {
			status = opsV1.ApiKey_DISABLED
		}
		dto.Status = &status
	}
	// 作用域:库存枚举值名,按 proto 枚举名转回;未知值跳过并告警(脏数据可见)
	for _, name := range e.Scopes {
		if v, ok := opsV1.ApiKey_Scope_value[name]; ok && v != 0 {
			dto.Scopes = append(dto.Scopes, opsV1.ApiKey_Scope(v))
		} else {
			r.log.Warnf("api key [%d] has unknown scope %q, skipped", e.ID, name)
		}
	}
	return dto
}

// List 分页列表。
func (r *ApiKeyRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApiKeyResponse, error) {
	client := r.entClient.Client().ApiKey

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(apikey.FieldID))
	if req != nil && (req.NoPaging == nil || !req.GetNoPaging()) {
		page := req.GetPage()
		if page < 1 {
			page = 1
		}
		pageSize := req.GetPageSize()
		if pageSize < 1 {
			pageSize = 50
		}
		qb = qb.Offset(int((page - 1) * pageSize)).Limit(int(pageSize))
	}

	rows, err := qb.All(ctx)
	if err != nil {
		return nil, err
	}

	resp := &opsV1.ListApiKeyResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// Get 详情。
func (r *ApiKeyRepo) Get(ctx context.Context, id uint32) (*opsV1.ApiKey, error) {
	e, err := r.entClient.Client().ApiKey.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// IsNameTaken 名字唯一性校验。
func (r *ApiKeyRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().ApiKey.Query().Where(apikey.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(apikey.IDNEQ(excludeID))
	}
	return query.Exist(ctx)
}

// Create 落库(明文由服务层生成,这里只收哈希与前缀;scopes 为 nil 存空=全部)。
func (r *ApiKeyRepo) Create(ctx context.Context, name, keyHash, prefix string, status opsV1.ApiKey_Status, description *string, expiresAt *time.Time, scopes []string, operatorID uint32) (*opsV1.ApiKey, error) {
	st := apikey.StatusEnabled
	if status == opsV1.ApiKey_DISABLED {
		st = apikey.StatusDisabled
	}
	builder := r.entClient.Client().ApiKey.Create().
		SetName(name).
		SetKeyHash(keyHash).
		SetPrefix(prefix).
		SetStatus(st)
	if len(scopes) > 0 {
		builder.SetScopes(scopes)
	}
	if description != nil {
		builder.SetDescription(*description)
	}
	if expiresAt != nil {
		builder.SetExpiresAt(*expiresAt)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Update 状态/描述/过期时间/作用域(scopes 非 nil 时覆盖,可传空切片清空=全部)。
func (r *ApiKeyRepo) Update(ctx context.Context, id uint32, status *opsV1.ApiKey_Status, description *string, expiresAt **time.Time, scopes *[]string) (*opsV1.ApiKey, error) {
	builder := r.entClient.Client().ApiKey.UpdateOneID(id)
	if status != nil {
		st := apikey.StatusEnabled
		if *status == opsV1.ApiKey_DISABLED {
			st = apikey.StatusDisabled
		}
		builder.SetStatus(st)
	}
	if description != nil {
		builder.SetDescription(*description)
	}
	if expiresAt != nil {
		builder.SetExpiresAt(**expiresAt)
	}
	if scopes != nil {
		if len(*scopes) == 0 {
			builder.ClearScopes()
		} else {
			builder.SetScopes(*scopes)
		}
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Delete 删除。
func (r *ApiKeyRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().ApiKey.DeleteOneID(id).Exec(ctx)
}

// GetByHash 按哈希取密钥(Validate 用;nil 表示不存在)。
func (r *ApiKeyRepo) GetByHash(ctx context.Context, keyHash string) (*ent.ApiKey, error) {
	return r.entClient.Client().ApiKey.Query().
		Where(apikey.KeyHashEQ(keyHash)).
		First(ctx)
}

// TouchLastUsed 异步回写最近使用时间。
// 调用方是 OpenAPI 网关(无操作者上下文);api_key 已挂租户隐私,注入系统
// viewer 避免无 viewer 上下文被 EvalMutation 拒绝。
func (r *ApiKeyRepo) TouchLastUsed(ctx context.Context, id uint32) {
	ctx = appViewer.NewSystemViewerContext(ctx)
	_, _ = r.entClient.Client().ApiKey.UpdateOneID(id).
		SetLastUsedAt(time.Now()).
		Save(ctx)
}
