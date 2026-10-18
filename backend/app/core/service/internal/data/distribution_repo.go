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
	"go-wind-ops/app/core/service/internal/data/ent/distribution"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type DistributionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper        *mapper.CopierMapper[opsV1.Distribution, ent.Distribution]
	typeConverter *mapper.EnumTypeConverter[opsV1.Distribution_AfterOpt, distribution.AfterOpt]

	repository *entCrud.Repository[
		ent.DistributionQuery, ent.DistributionSelect,
		ent.DistributionCreate, ent.DistributionCreateBulk,
		ent.DistributionUpdate, ent.DistributionUpdateOne,
		ent.DistributionDelete,
		predicate.Distribution,
		opsV1.Distribution, ent.Distribution,
	]
}

func NewDistributionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *DistributionRepo {
	repo := &DistributionRepo{
		log:           ctx.NewLoggerHelper("distribution/repo/core-service"),
		entClient:     entClient,
		mapper:        mapper.NewCopierMapper[opsV1.Distribution, ent.Distribution](),
		typeConverter: mapper.NewEnumTypeConverter[opsV1.Distribution_AfterOpt, distribution.AfterOpt](opsV1.Distribution_AfterOpt_name, opsV1.Distribution_AfterOpt_value),
	}

	repo.init()

	return repo
}

func (r *DistributionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.DistributionQuery, ent.DistributionSelect,
		ent.DistributionCreate, ent.DistributionCreateBulk,
		ent.DistributionUpdate, ent.DistributionUpdateOne,
		ent.DistributionDelete,
		predicate.Distribution,
		opsV1.Distribution, ent.Distribution,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *DistributionRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Distribution.Query()
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

func (r *DistributionRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Distribution.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListDistributionResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListDistributionResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *DistributionRepo) Get(ctx context.Context, req *opsV1.GetDistributionRequest) (*opsV1.Distribution, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Distribution.Query()

	dto, err := r.repository.Get(ctx, builder, nil, distribution.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("distribution not found")
		}
		r.log.Errorf("query distribution [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// IsNameTaken 校验分发名唯一性。
func (r *DistributionRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Distribution.Query().Where(distribution.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(distribution.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query distribution name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *DistributionRepo) Create(ctx context.Context, req *opsV1.CreateDistributionRequest, operatorID uint32) (*opsV1.Distribution, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Distribution.Create().
		SetName(req.Data.GetName()).
		SetBuildID(req.Data.GetBuildId()).
		SetProjectID(req.Data.GetProjectId()).
		SetClearOld(req.Data.GetClearOld()).
		SetNillableDescription(req.Data.Description).
		SetLabels(req.Data.Labels)

	if req.Data.AfterOpt != nil {
		builder.SetAfterOpt(afterOptToEntity(req.Data.GetAfterOpt()))
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert distribution failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *DistributionRepo) Update(ctx context.Context, req *opsV1.UpdateDistributionRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Distribution.UpdateOneID(req.GetId())
	builder.Where(distribution.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Distribution) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetClearOld(req.Data.GetClearOld())

			if req.Data.BuildId != nil && req.Data.GetBuildId() > 0 {
				builder.SetBuildID(req.Data.GetBuildId())
			}
			if req.Data.ProjectId != nil && req.Data.GetProjectId() > 0 {
				builder.SetProjectID(req.Data.GetProjectId())
			}
			if req.Data.AfterOpt != nil {
				builder.SetAfterOpt(afterOptToEntity(req.Data.GetAfterOpt()))
			}
			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(distribution.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *DistributionRepo) Delete(ctx context.Context, req *opsV1.DeleteDistributionRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Distribution.Delete()
	delBuilder.Where(distribution.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("distribution not found")
		}
		r.log.Errorf("delete distribution [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// afterOptToEntity proto 枚举 → ent 枚举(ent 存 NONE/RESTART/START)。
func afterOptToEntity(v opsV1.Distribution_AfterOpt) distribution.AfterOpt {
	switch v {
	case opsV1.Distribution_RESTART:
		return distribution.AfterOptRestart
	case opsV1.Distribution_START:
		return distribution.AfterOptStart
	default:
		return distribution.AfterOptNone
	}
}
