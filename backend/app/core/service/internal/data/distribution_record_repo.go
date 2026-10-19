package data

import (
	"context"
	"fmt"
	"strings"
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

// pendingRequestPrefix 未投递批次的占位幂等键前缀:记录先建(批次视图完整),
// 投递时以"占位键 → 真实 UUID"的条件更新实现多副本抢占,恰一方获得投递权。
const pendingRequestPrefix = "pending:"

// MakePendingRequestID 生成未投递批次的占位幂等键(唯一索引保证逐记录唯一)。
func MakePendingRequestID(distributionID, batchNo, projectID uint32) string {
	return fmt.Sprintf("%s%d:%d:%d", pendingRequestPrefix, distributionID, batchNo, projectID)
}

// IsPendingRequestID 判断是否未投递批次的占位幂等键。
func IsPendingRequestID(requestID string) bool {
	return strings.HasPrefix(requestID, pendingRequestPrefix)
}

func (r *DistributionRecordRepo) Create(ctx context.Context, distributionID, historyID, projectID, nodeID uint32, commandRequestID string, operatorID uint32, batchNo uint32) (*ent.DistributionRecord, error) {
	if batchNo == 0 {
		batchNo = 1
	}
	builder := r.entClient.Client().DistributionRecord.
		Create().
		SetDistributionID(distributionID).
		SetBuildHistoryID(historyID).
		SetProjectID(projectID).
		SetNodeID(nodeID).
		SetCommandRequestID(commandRequestID).
		SetBatchNo(batchNo).
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

// ListByDistribution 列出某分发配置的全部执行记录(批次推进判断用)。
func (r *DistributionRecordRepo) ListByDistribution(ctx context.Context, distributionID uint32) ([]*ent.DistributionRecord, error) {
	return r.entClient.Client().DistributionRecord.
		Query().
		Where(distributionrecord.DistributionIDEQ(distributionID)).
		Order(ent.Asc(distributionrecord.FieldBatchNo), ent.Asc(distributionrecord.FieldID)).
		All(ctx)
}

// ClaimForDispatch 把占位记录领取为真实投递:仅当记录仍是该占位键且
// PENDING 时写入真实指令幂等键,返回是否由本次调用获得投递权(多副本
// 并发推进时恰一方为 true,批次不会重复下发)。
func (r *DistributionRecordRepo) ClaimForDispatch(ctx context.Context, recordID uint32, placeholder, requestID string) (bool, error) {
	n, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.IDEQ(recordID),
			distributionrecord.CommandRequestIDEQ(placeholder),
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
		).
		SetCommandRequestID(requestID).
		Save(ctx)
	if err != nil {
		r.log.Errorf("claim distribution record [%d]: %s", recordID, err.Error())
		return false, err
	}
	return n > 0, nil
}

// SkipPending 把未投递的 PENDING 记录置为 SKIPPED(前序批次失败自动中止),
// 幂等(仅 PENDING 可跳过);返回是否发生置位。
func (r *DistributionRecordRepo) SkipPending(ctx context.Context, recordID uint32, reason string) (bool, error) {
	now := time.Now()
	n, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.IDEQ(recordID),
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
			distributionrecord.CommandRequestIDHasPrefix(pendingRequestPrefix),
		).
		SetStatus(distributionrecord.StatusSkipped).
		SetNillableError(ptrOrNil(reason)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	if err != nil {
		r.log.Errorf("skip distribution record [%d]: %s", recordID, err.Error())
		return false, err
	}
	return n > 0, nil
}

// SkipAllPendingByDistribution 把该分发配置下全部未投递占位记录置 SKIPPED
// (重新触发即取代旧 rollout,防占位记录永久滞留 PENDING)。
func (r *DistributionRecordRepo) SkipAllPendingByDistribution(ctx context.Context, distributionID uint32, reason string) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.DistributionIDEQ(distributionID),
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
			distributionrecord.CommandRequestIDHasPrefix(pendingRequestPrefix),
		).
		SetStatus(distributionrecord.StatusSkipped).
		SetNillableError(ptrOrNil(reason)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	return n, err
}

// FailPending 把占位记录直接置为 FAILED(批次内投递失败,未发出指令)。
func (r *DistributionRecordRepo) FailPending(ctx context.Context, recordID uint32, errMsg string) error {
	now := time.Now()
	_, err := r.entClient.Client().DistributionRecord.
		Update().
		Where(
			distributionrecord.IDEQ(recordID),
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
			distributionrecord.CommandRequestIDHasPrefix(pendingRequestPrefix),
		).
		SetStatus(distributionrecord.StatusFailed).
		SetNillableError(ptrOrNil(errMsg)).
		SetNillableFinishedAt(&now).
		Save(ctx)
	return err
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

// FinalizeStalePending 把超时未回执的分发记录定案 FAILED(指令超时驱动),
// 返回受影响的分发配置 ID(供批次推进钩子复判)。
func (r *DistributionRecordRepo) FinalizeStalePending(ctx context.Context, before time.Time) ([]uint32, error) {
	stale, err := r.entClient.Client().DistributionRecord.
		Query().
		Where(
			distributionrecord.StatusEQ(distributionrecord.StatusPending),
			distributionrecord.CreatedAtLT(before),
			distributionrecord.Not(distributionrecord.CommandRequestIDHasPrefix(pendingRequestPrefix)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(stale) == 0 {
		return nil, nil
	}
	now := time.Now()
	distIDs := make([]uint32, 0, len(stale))
	for _, rec := range stale {
		n, err := r.entClient.Client().DistributionRecord.
			Update().
			Where(
				distributionrecord.IDEQ(rec.ID),
				distributionrecord.StatusEQ(distributionrecord.StatusPending),
			).
			SetStatus(distributionrecord.StatusFailed).
			SetError("distribution timeout without result").
			SetNillableFinishedAt(&now).
			Save(ctx)
		if err != nil {
			r.log.Errorf("finalize stale distribution record [%d]: %s", rec.ID, err.Error())
			continue
		}
		if n > 0 {
			distIDs = append(distIDs, rec.DistributionID)
		}
	}
	return distIDs, nil
}
