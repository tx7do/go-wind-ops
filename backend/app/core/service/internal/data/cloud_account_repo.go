package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/cloudaccount"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudAccountRepo 云平台账号仓储(手写转换,同 api_key 范式)。
// 实体含密文 SK,toDTO 只出掩码,明文永不进入 DTO。
type CloudAccountRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewCloudAccountRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CloudAccountRepo {
	return &CloudAccountRepo{
		log:       ctx.NewLoggerHelper("cloud-account/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO 实体 → proto(SK 仅出固定掩码)。
func (r *CloudAccountRepo) toDTO(e *ent.CloudAccount) *opsV1.CloudAccount {
	dto := &opsV1.CloudAccount{
		Id:             &e.ID,
		Name:           &e.Name,
		AccountId:      e.AccountID,
		AccountName:    e.AccountName,
		AccessKey:      e.AccessKey,
		Region:         e.Region,
		Description:    e.Description,
		CreatedAt:      timeTimestamppb(e.CreatedAt),
		LastVerifiedAt: timeTimestamppb(e.LastVerifiedAt),
	}
	if e.Provider != nil {
		provider := providerToProto(*e.Provider)
		dto.Provider = &provider
	}
	if e.Status != nil {
		status := opsV1.CloudAccount_ENABLED
		if *e.Status == cloudaccount.StatusDisabled {
			status = opsV1.CloudAccount_DISABLED
		}
		dto.Status = &status
	}
	if e.SecretKey != nil && *e.SecretKey != "" {
		mask := "••••••"
		dto.SecretMask = &mask
	}
	return dto
}

// providerToProto ent 枚举 → proto 枚举。
func providerToProto(p cloudaccount.Provider) opsV1.CloudAccount_Provider {
	switch p {
	case cloudaccount.ProviderTencentCloud:
		return opsV1.CloudAccount_TENCENT_CLOUD
	case cloudaccount.ProviderHuaweiCloud:
		return opsV1.CloudAccount_HUAWEI_CLOUD
	case cloudaccount.ProviderAWS:
		return opsV1.CloudAccount_AWS
	default:
		return opsV1.CloudAccount_ALIYUN
	}
}

// providerToEnt proto 枚举 → ent 枚举。
func providerToEnt(p opsV1.CloudAccount_Provider) cloudaccount.Provider {
	switch p {
	case opsV1.CloudAccount_TENCENT_CLOUD:
		return cloudaccount.ProviderTencentCloud
	case opsV1.CloudAccount_HUAWEI_CLOUD:
		return cloudaccount.ProviderHuaweiCloud
	case opsV1.CloudAccount_AWS:
		return cloudaccount.ProviderAWS
	default:
		return cloudaccount.ProviderAliyun
	}
}

// List 分页列表。
func (r *CloudAccountRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudAccountsResponse, error) {
	client := r.entClient.Client().CloudAccount

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(cloudaccount.FieldID))
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

	resp := &opsV1.ListCloudAccountsResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// Get 详情。
func (r *CloudAccountRepo) Get(ctx context.Context, id uint32) (*opsV1.CloudAccount, error) {
	e, err := r.GetEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// GetEntity 原始实体(Verify 需读密文 SK)。
func (r *CloudAccountRepo) GetEntity(ctx context.Context, id uint32) (*ent.CloudAccount, error) {
	return r.entClient.Client().CloudAccount.Get(ctx, id)
}

// IsNameTaken 名字唯一性校验。
func (r *CloudAccountRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().CloudAccount.Query().Where(cloudaccount.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(cloudaccount.IDNEQ(excludeID))
	}
	return query.Exist(ctx)
}

// Create 落库(SK 密文由服务层加密后传入)。
func (r *CloudAccountRepo) Create(ctx context.Context, name string, provider opsV1.CloudAccount_Provider, accessKey, secretEncrypted, region string, status opsV1.CloudAccount_Status, description *string, operatorID uint32) (*opsV1.CloudAccount, error) {
	st := cloudaccount.StatusEnabled
	if status == opsV1.CloudAccount_DISABLED {
		st = cloudaccount.StatusDisabled
	}
	builder := r.entClient.Client().CloudAccount.Create().
		SetName(name).
		SetProvider(providerToEnt(provider)).
		SetStatus(st)
	if accessKey != "" {
		builder = builder.SetAccessKey(accessKey)
	}
	if secretEncrypted != "" {
		builder = builder.SetSecretKey(secretEncrypted)
	}
	if region != "" {
		builder = builder.SetRegion(region)
	}
	if description != nil {
		builder.SetDescription(*description)
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

// Update 基础字段;secretEncrypted 非空才换密钥。
func (r *CloudAccountRepo) Update(ctx context.Context, id uint32, name string, provider *opsV1.CloudAccount_Provider, accessKey *string, secretEncrypted *string, region *string, status *opsV1.CloudAccount_Status, description *string) (*opsV1.CloudAccount, error) {
	builder := r.entClient.Client().CloudAccount.UpdateOneID(id)
	if name != "" {
		builder.SetName(name)
	}
	if provider != nil {
		builder.SetProvider(providerToEnt(*provider))
	}
	if accessKey != nil {
		builder.SetAccessKey(*accessKey)
	}
	if secretEncrypted != nil && *secretEncrypted != "" {
		builder.SetSecretKey(*secretEncrypted)
	}
	if region != nil {
		builder.SetRegion(*region)
	}
	if status != nil {
		st := cloudaccount.StatusEnabled
		if *status == opsV1.CloudAccount_DISABLED {
			st = cloudaccount.StatusDisabled
		}
		builder.SetStatus(st)
	}
	if description != nil {
		builder.SetDescription(*description)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// UpdateVerifyResult 校验结果回填(账号 ID/名 + 校验时间)。
func (r *CloudAccountRepo) UpdateVerifyResult(ctx context.Context, id uint32, accountID, accountName string, verifiedAt time.Time) error {
	builder := r.entClient.Client().CloudAccount.UpdateOneID(id).
		SetLastVerifiedAt(verifiedAt)
	if accountID != "" {
		builder = builder.SetAccountID(accountID)
	}
	if accountName != "" {
		builder = builder.SetAccountName(accountName)
	}
	return builder.Exec(ctx)
}

// Delete 删除。
func (r *CloudAccountRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().CloudAccount.DeleteOneID(id).Exec(ctx)
}
