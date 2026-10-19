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
	"go-wind-ops/app/core/service/internal/data/ent/commandrecord"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type CommandRecordRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper        *mapper.CopierMapper[opsV1.CommandRecord, ent.CommandRecord]
	typeConverter *mapper.EnumTypeConverter[opsV1.CommandStatus, commandrecord.Status]

	repository *entCrud.Repository[
		ent.CommandRecordQuery, ent.CommandRecordSelect,
		ent.CommandRecordCreate, ent.CommandRecordCreateBulk,
		ent.CommandRecordUpdate, ent.CommandRecordUpdateOne,
		ent.CommandRecordDelete,
		predicate.CommandRecord,
		opsV1.CommandRecord, ent.CommandRecord,
	]
}

func NewCommandRecordRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CommandRecordRepo {
	repo := &CommandRecordRepo{
		log:           ctx.NewLoggerHelper("command-record/repo/core-service"),
		entClient:     entClient,
		mapper:        mapper.NewCopierMapper[opsV1.CommandRecord, ent.CommandRecord](),
		typeConverter: mapper.NewEnumTypeConverter[opsV1.CommandStatus, commandrecord.Status](opsV1.CommandStatus_name, opsV1.CommandStatus_value),
	}

	repo.init()

	return repo
}

func (r *CommandRecordRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.CommandRecordQuery, ent.CommandRecordSelect,
		ent.CommandRecordCreate, ent.CommandRecordCreateBulk,
		ent.CommandRecordUpdate, ent.CommandRecordUpdateOne,
		ent.CommandRecordDelete,
		predicate.CommandRecord,
		opsV1.CommandRecord, ent.CommandRecord,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *CommandRecordRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().CommandRecord.Query()
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

func (r *CommandRecordRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCommandRecordResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CommandRecord.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListCommandRecordResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListCommandRecordResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *CommandRecordRepo) Get(ctx context.Context, req *opsV1.GetCommandRecordRequest) (*opsV1.CommandRecord, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().CommandRecord.Query()

	dto, err := r.repository.Get(ctx, builder, nil, commandrecord.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("command record not found")
		}
		r.log.Errorf("query command record [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// Create 指令下发时创建 PENDING 台账行。
func (r *CommandRecordRepo) Create(ctx context.Context, requestID string, nodeID uint32, agentID, action, payload string, operatorID uint32) (*ent.CommandRecord, error) {
	builder := r.entClient.Client().CommandRecord.
		Create().
		SetRequestID(requestID).
		SetAgentID(agentID).
		SetAction(action).
		SetStatus(commandrecord.StatusPending)

	if nodeID > 0 {
		builder.SetNodeID(nodeID)
	}
	if payload != "" {
		builder.SetPayload(payload)
	}
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create command record [%s]: %s", requestID, err.Error())
		return nil, err
	}
	return record, nil
}

// GetByRequestID 按幂等键取指令记录(流水线脚本步骤轮询终态用)。
func (r *CommandRecordRepo) GetByRequestID(ctx context.Context, requestID string) (*ent.CommandRecord, error) {
	return r.entClient.Client().CommandRecord.Query().
		Where(commandrecord.RequestIDEQ(requestID)).
		First(ctx)
}

// FinalizeByRequestID 按幂等键回填台账终态;记录不存在或已终态时静默返回
// (重复回执/手工注入场景)。
func (r *CommandRecordRepo) FinalizeByRequestID(ctx context.Context, requestID, status, errMsg, result string) error {
	now := time.Now()
	n, err := r.entClient.Client().CommandRecord.
		Update().
		Where(
			commandrecord.RequestIDEQ(requestID),
			commandrecord.StatusEQ(commandrecord.StatusPending),
		).
		SetStatus(commandrecord.Status(status)).
		SetNillableError(ptrOrNil(errMsg)).
		SetNillableResult(ptrOrNil(result)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		r.log.Debugf("command record [%s] not pending (duplicated or unknown)", requestID)
	}
	return nil
}

// ptrOrNil 空串转 nil(可选文本字段不覆盖原值)。
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// AttachBatch 给指令台账打上批量执行批次标记(批次先建、指令后发,事后回填)。
func (r *CommandRecordRepo) AttachBatch(ctx context.Context, requestID string, batchID uint32) error {
	_, err := r.entClient.Client().CommandRecord.
		Update().
		Where(
			commandrecord.RequestIDEQ(requestID),
			commandrecord.BatchIDEQ(0),
		).
		SetBatchID(batchID).
		Save(ctx)
	return err
}

// ListStalePendingBatchItems 取超时未回执且带批次标记的 PENDING 台账
// (批次计数对账用;定案由调用方逐条执行)。
func (r *CommandRecordRepo) ListStalePendingBatchItems(ctx context.Context, before time.Time) ([]*ent.CommandRecord, error) {
	return r.entClient.Client().CommandRecord.
		Query().
		Where(
			commandrecord.StatusEQ(commandrecord.StatusPending),
			commandrecord.BatchIDGT(0),
			commandrecord.CreatedAtLT(before),
		).
		All(ctx)
}

// ListByBatch 取批次内全部指令台账(按下发顺序)。
func (r *CommandRecordRepo) ListByBatch(ctx context.Context, batchID uint32) ([]*ent.CommandRecord, error) {
	return r.entClient.Client().CommandRecord.
		Query().
		Where(commandrecord.BatchIDEQ(batchID)).
		Order(ent.Asc(commandrecord.FieldID)).
		All(ctx)
}

// FinalizeStalePending 把超时未回执的 PENDING 台账定案 FAILED,返回行数。
// 幂等,可多副本并发执行。
func (r *CommandRecordRepo) FinalizeStalePending(ctx context.Context, before time.Time) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().CommandRecord.
		Update().
		Where(
			commandrecord.StatusEQ(commandrecord.StatusPending),
			commandrecord.CreatedAtLT(before),
		).
		SetStatus(commandrecord.StatusFailed).
		SetError("command timeout without result").
		SetNillableFinishedAt(&now).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return n, nil
}
