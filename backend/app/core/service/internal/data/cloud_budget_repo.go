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
	"go-wind-ops/app/core/service/internal/data/ent/cloudbudget"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type CloudBudgetRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.CloudBudget, ent.CloudBudget]

	repository *entCrud.Repository[
		ent.CloudBudgetQuery, ent.CloudBudgetSelect,
		ent.CloudBudgetCreate, ent.CloudBudgetCreateBulk,
		ent.CloudBudgetUpdate, ent.CloudBudgetUpdateOne,
		ent.CloudBudgetDelete,
		predicate.CloudBudget,
		opsV1.CloudBudget, ent.CloudBudget,
	]
}

func NewCloudBudgetRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CloudBudgetRepo {
	repo := &CloudBudgetRepo{
		log:       ctx.NewLoggerHelper("cloud-budget/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.CloudBudget, ent.CloudBudget](),
	}

	repo.init()

	return repo
}

func (r *CloudBudgetRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.CloudBudgetQuery, ent.CloudBudgetSelect,
		ent.CloudBudgetCreate, ent.CloudBudgetCreateBulk,
		ent.CloudBudgetUpdate, ent.CloudBudgetUpdateOne,
		ent.CloudBudgetDelete,
		predicate.CloudBudget,
		opsV1.CloudBudget, ent.CloudBudget,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *CloudBudgetRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudBudgetResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CloudBudget.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListCloudBudgetResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListCloudBudgetResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabled 取启用的预算(调度检查用)。
func (r *CloudBudgetRepo) ListEnabled(ctx context.Context) ([]*ent.CloudBudget, error) {
	return r.entClient.Client().CloudBudget.
		Query().
		Where(cloudbudget.EnabledEQ(true)).
		All(ctx)
}

func (r *CloudBudgetRepo) Get(ctx context.Context, req *opsV1.GetCloudBudgetRequest) (*opsV1.CloudBudget, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CloudBudget.Query()

	dto, err := r.repository.Get(ctx, builder, nil, cloudbudget.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("cloud budget not found")
		}
		r.log.Errorf("query cloud budget [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(检查用)。
func (r *CloudBudgetRepo) GetEntity(ctx context.Context, id uint32) (*ent.CloudBudget, error) {
	return r.entClient.Client().CloudBudget.
		Query().
		Where(cloudbudget.IDEQ(id)).
		First(ctx)
}

// IsKeyTaken 同账号同月份预算唯一性校验。
func (r *CloudBudgetRepo) IsKeyTaken(ctx context.Context, accountID uint32, month string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().CloudBudget.Query().
		Where(
			cloudbudget.CloudAccountIDEQ(accountID),
			cloudbudget.MonthEQ(month),
		)
	if excludeID > 0 {
		query = query.Where(cloudbudget.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query cloud budget taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *CloudBudgetRepo) Create(ctx context.Context, req *opsV1.CreateCloudBudgetRequest, operatorID uint32) (*opsV1.CloudBudget, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CloudBudget.Create().
		SetCloudAccountID(req.Data.GetCloudAccountId()).
		SetMonth(req.Data.GetMonth()).
		SetAmount(req.Data.GetAmount()).
		SetCurrency(req.Data.GetCurrency()).
		SetThresholdPercent(req.Data.GetThresholdPercent()).
		SetAnomalyRatio(req.Data.GetAnomalyRatio()).
		SetEnabled(req.Data.GetEnabled()).
		SetNillableDescription(req.Data.Description)

	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert cloud budget failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *CloudBudgetRepo) Update(ctx context.Context, req *opsV1.UpdateCloudBudgetRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CloudBudget.UpdateOneID(req.GetId())
	builder.Where(cloudbudget.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.CloudBudget) {
			builder.
				SetAmount(req.Data.GetAmount()).
				SetCurrency(req.Data.GetCurrency()).
				SetThresholdPercent(req.Data.GetThresholdPercent()).
				SetAnomalyRatio(req.Data.GetAnomalyRatio()).
				SetEnabled(req.Data.GetEnabled()).
				SetNillableDescription(req.Data.Description)

			if req.Data.Month != nil && req.Data.GetMonth() != "" {
				builder.SetMonth(req.Data.GetMonth())
			}
			if req.Data.CloudAccountId != nil {
				builder.SetCloudAccountID(req.Data.GetCloudAccountId())
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(cloudbudget.FieldID, req.GetId()))
		},
	)

	return err
}

// SetNotified 阈值告警防重位。
func (r *CloudBudgetRepo) SetNotified(ctx context.Context, id uint32, notified bool) error {
	_, err := r.entClient.Client().CloudBudget.
		UpdateOneID(id).
		SetNotified(notified).
		Save(ctx)
	return err
}

// SetAnomalyNotified 突增告警防重位。
func (r *CloudBudgetRepo) SetAnomalyNotified(ctx context.Context, id uint32, notified bool) error {
	_, err := r.entClient.Client().CloudBudget.
		UpdateOneID(id).
		SetAnomalyNotified(notified).
		Save(ctx)
	return err
}

func (r *CloudBudgetRepo) Delete(ctx context.Context, req *opsV1.DeleteCloudBudgetRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().CloudBudget.Delete()
	delBuilder.Where(cloudbudget.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("cloud budget not found")
		}
		r.log.Errorf("delete cloud budget [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// Count 按条件计数(预留)。
func (r *CloudBudgetRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().CloudBudget.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}
	return builder.Count(ctx)
}
