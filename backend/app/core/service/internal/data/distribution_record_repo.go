package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/distributionrecord"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type DistributionRecordRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.DistributionRecord, ent.DistributionRecord]

	repository *entCrud.Repository[
		ent.DistributionRecordQuery, ent.DistributionRecordSelect,
		ent.DistributionRecordCreate, ent.DistributionRecordCreateBulk,
		ent.DistributionRecordUpdate, ent.DistributionRecordUpdateOne,
		ent.DistributionRecordDelete,
		predicate.DistributionRecord,
		opsV1.DistributionRecord, ent.DistributionRecord,
	]
}

func NewDistributionRecordRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *DistributionRecordRepo {
	repo := &DistributionRecordRepo{
		log:       ctx.NewLoggerHelper("distribution-record/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.DistributionRecord, ent.DistributionRecord](),
	}

	repo.init()

	return repo
}

func (r *DistributionRecordRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.DistributionRecordQuery, ent.DistributionRecordSelect,
		ent.DistributionRecordCreate, ent.DistributionRecordCreateBulk,
		ent.DistributionRecordUpdate, ent.DistributionRecordUpdateOne,
		ent.DistributionRecordDelete,
		predicate.DistributionRecord,
		opsV1.DistributionRecord, ent.DistributionRecord,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *DistributionRecordRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().DistributionRecord.Query()
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

func (r *DistributionRecordRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionRecordResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().DistributionRecord.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListDistributionRecordResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListDistributionRecordResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *DistributionRecordRepo) Get(ctx context.Context, req *opsV1.GetDistributionRecordRequest) (*opsV1.DistributionRecord, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().DistributionRecord.Query()

	dto, err := r.repository.Get(ctx, builder, nil, distributionrecord.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("distribution record not found")
		}
		r.log.Errorf("query distribution record [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

func (r *DistributionRecordRepo) Create(ctx context.Context, distributionID, historyID, projectID, nodeID uint32, commandRequestID string, operatorID uint32) (*ent.DistributionRecord, error) {
	builder := r.entClient.Client().DistributionRecord.
		Create().
		SetDistributionID(distributionID).
		SetBuildHistoryID(historyID).
		SetProjectID(projectID).
		SetNodeID(nodeID).
		SetCommandRequestID(commandRequestID).
		SetStatus(distributionrecord.StatusPending)

	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create distribution record: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create record failed")
	}
	return record, nil
}

// GetEntity 取原始实体。
func (r *DistributionRecordRepo) GetEntity(ctx context.Context, id uint32) (*ent.DistributionRecord, error) {
	entity, err := r.entClient.Client().DistributionRecord.
		Query().
		Where(distributionrecord.IDEQ(id)).
		First(ctx)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

// GetByCommandRequestID 按关联指令幂等键取记录(MQTT 回执对账)。
func (r *DistributionRecordRepo) GetByCommandRequestID(ctx context.Context, requestID string) (*ent.DistributionRecord, error) {
	entity, err := r.entClient.Client().DistributionRecord.
		Query().
		Where(distributionrecord.CommandRequestIDEQ(requestID)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entity, nil
}

// FinalizeByCommandRequestID 按指令幂等键回填分发记录终态;记录不存在或
// 已终态时静默返回。
func (r *DistributionRecordRepo) FinalizeByCommandRequestID(ctx context.Context, requestID, status, errMsg, result string) error {
	now := time.Now()
	n, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.CommandRequestIDEQ(requestID),
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
		).
		SetStatus(distributionrecord.Status(status)).
		SetNillableError(ptrOrNil(errMsg)).
		SetNillableResult(ptrOrNil(result)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		r.log.Debugf("distribution record [%s] not pending (duplicated or unknown)", requestID)
	}
	return nil
}

// FinalizeStalePending 把超时未回执的分发记录定案 FAILED(指令超时驱动)。
func (r *DistributionRecordRepo) FinalizeStalePending(ctx context.Context, before time.Time) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
			distributionrecord.CreatedAtLT(before),
		).
		SetStatus(distributionrecord.StatusFailed).
		SetError("distribution timeout without result").
		SetNillableFinishedAt(&now).
		Save(ctx)
	return n, err
}
