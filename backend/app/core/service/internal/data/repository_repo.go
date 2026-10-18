package data

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"
	"go-wind-ops/app/core/service/internal/data/ent/repository"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// PasswordMask 仓库密码回显掩码。
const PasswordMask = "******"

type RepositoryRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Repository, ent.Repository]

	repository *entCrud.Repository[
		ent.RepositoryQuery, ent.RepositorySelect,
		ent.RepositoryCreate, ent.RepositoryCreateBulk,
		ent.RepositoryUpdate, ent.RepositoryUpdateOne,
		ent.RepositoryDelete,
		predicate.Repository,
		opsV1.Repository, ent.Repository,
	]
}

func NewRepositoryRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *RepositoryRepo {
	repo := &RepositoryRepo{
		log:       ctx.NewLoggerHelper("repository/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Repository, ent.Repository](),
	}

	repo.init()

	return repo
}

func (r *RepositoryRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.RepositoryQuery, ent.RepositorySelect,
		ent.RepositoryCreate, ent.RepositoryCreateBulk,
		ent.RepositoryUpdate, ent.RepositoryUpdateOne,
		ent.RepositoryDelete,
		predicate.Repository,
		opsV1.Repository, ent.Repository,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *RepositoryRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Repository.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, opsV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func (r *RepositoryRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListRepositoryResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Repository.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListRepositoryResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListRepositoryResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *RepositoryRepo) Get(ctx context.Context, req *opsV1.GetRepositoryRequest) (*opsV1.Repository, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Repository.Query()

	dto, err := r.repository.Get(ctx, builder, nil, repository.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("repository not found")
		}
		r.log.Errorf("query repository [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(含密文密码,供构建执行器解密使用)。
func (r *RepositoryRepo) GetEntity(ctx context.Context, id uint32) (*ent.Repository, error) {
	entity, err := r.entClient.Client().Repository.
		Query().
		Where(repository.IDEQ(id)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, opsV1.ErrorNotFound("repository not found")
	}
	if err != nil {
		r.log.Errorf("query repository entity [%d]: %s", id, err.Error())
		return nil, err
	}
	return entity, nil
}

// IsNameTaken 校验仓库名唯一性。
func (r *RepositoryRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Repository.Query().Where(repository.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(repository.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query repository name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *RepositoryRepo) Create(ctx context.Context, req *opsV1.CreateRepositoryRequest, operatorID uint32) (*opsV1.Repository, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Repository.Create().
		SetName(req.Data.GetName()).
		SetGitURL(req.Data.GetGitUrl()).
		SetNillableDescription(req.Data.Description).
		SetLabels(req.Data.Labels)

	if req.Data.Branch != nil {
		builder.SetBranch(req.Data.GetBranch())
	}
	if req.Data.Username != nil {
		builder.SetUsername(req.Data.GetUsername())
	}
	if req.Data.Password != nil {
		builder.SetPassword(req.Data.GetPassword())
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert repository failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *RepositoryRepo) Update(ctx context.Context, req *opsV1.UpdateRepositoryRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Repository.UpdateOneID(req.GetId())
	builder.Where(repository.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Repository) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableGitURL(req.Data.GitUrl).
				SetNillableBranch(req.Data.Branch).
				SetNillableUsername(req.Data.Username).
				SetNillableDescription(req.Data.Description)

			// 密码仅在显式提供且非掩码时更新
			if pw := req.Data.GetPassword(); pw != "" && pw != PasswordMask {
				builder.SetPassword(pw)
			}
			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(repository.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *RepositoryRepo) Delete(ctx context.Context, req *opsV1.DeleteRepositoryRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Repository.Delete()
	delBuilder.Where(repository.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("repository not found")
		}
		r.log.Errorf("delete repository [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// MaskPassword 把 DTO 中的密码替换为掩码(对外回显)。
func MaskPassword(dto *opsV1.Repository) *opsV1.Repository {
	if dto != nil && dto.Password != nil {
		dto.Password = ptrOrNil(PasswordMask)
	}
	return dto
}
