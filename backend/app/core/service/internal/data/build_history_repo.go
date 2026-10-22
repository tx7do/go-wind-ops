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
	"go-wind-ops/app/core/service/internal/data/ent/build"
	"go-wind-ops/app/core/service/internal/data/ent/buildhistory"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type BuildHistoryRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper        *mapper.CopierMapper[opsV1.BuildHistory, ent.BuildHistory]
	typeConverter *mapper.EnumTypeConverter[opsV1.BuildHistory_Status, buildhistory.Status]

	repository *entCrud.Repository[
		ent.BuildHistoryQuery, ent.BuildHistorySelect,
		ent.BuildHistoryCreate, ent.BuildHistoryCreateBulk,
		ent.BuildHistoryUpdate, ent.BuildHistoryUpdateOne,
		ent.BuildHistoryDelete,
		predicate.BuildHistory,
		opsV1.BuildHistory, ent.BuildHistory,
	]
}

func NewBuildHistoryRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BuildHistoryRepo {
	repo := &BuildHistoryRepo{
		log:           ctx.NewLoggerHelper("build-history/repo/core-service"),
		entClient:     entClient,
		mapper:        mapper.NewCopierMapper[opsV1.BuildHistory, ent.BuildHistory](),
		typeConverter: mapper.NewEnumTypeConverter[opsV1.BuildHistory_Status, buildhistory.Status](opsV1.BuildHistory_Status_name, opsV1.BuildHistory_Status_value),
	}

	repo.init()

	return repo
}

func (r *BuildHistoryRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.BuildHistoryQuery, ent.BuildHistorySelect,
		ent.BuildHistoryCreate, ent.BuildHistoryCreateBulk,
		ent.BuildHistoryUpdate, ent.BuildHistoryUpdateOne,
		ent.BuildHistoryDelete,
		predicate.BuildHistory,
		opsV1.BuildHistory, ent.BuildHistory,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *BuildHistoryRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().BuildHistory.Query()
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

func (r *BuildHistoryRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildHistoryResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BuildHistory.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListBuildHistoryResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListBuildHistoryResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *BuildHistoryRepo) Get(ctx context.Context, req *opsV1.GetBuildHistoryRequest) (*opsV1.BuildHistory, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BuildHistory.Query()

	dto, err := r.repository.Get(ctx, builder, nil, buildhistory.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("build history not found")
		}
		r.log.Errorf("query build history [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// ListArtifactHoldersByBuild 返回某构建配置下,保留数量之外仍持有产物
// 对象键的成功历史(按构建号倒序,跳过最新 keep 条)。保留策略清扫用。
func (r *BuildHistoryRepo) ListArtifactHoldersByBuild(ctx context.Context, buildID uint32, keep uint32) ([]*ent.BuildHistory, error) {
	rows, err := r.entClient.Client().BuildHistory.Query().
		Where(
			buildhistory.BuildIDEQ(buildID),
			buildhistory.StatusEQ(buildhistory.StatusSuccess),
			buildhistory.ArtifactObjectKeyNEQ(""),
			buildhistory.ArtifactObjectKeyNotNil(),
		).
		Order(ent.Desc(buildhistory.FieldBuildNo)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if int(keep) >= len(rows) {
		return nil, nil
	}
	return rows[keep:], nil
}

// ClearArtifact 清空历史记录的产物指针(对象已从对象存储删除)。
func (r *BuildHistoryRepo) ClearArtifact(ctx context.Context, id uint32) error {
	_, err := r.entClient.Client().BuildHistory.UpdateOneID(id).
		ClearArtifactObjectKey().
		ClearArtifactSize().
		Save(ctx)
	return err
}

// GetEntity 取原始实体(构建执行器用)。
func (r *BuildHistoryRepo) GetEntity(ctx context.Context, id uint32) (*ent.BuildHistory, error) {
	entity, err := r.entClient.Client().BuildHistory.
		Query().
		Where(buildhistory.IDEQ(id)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, opsV1.ErrorNotFound("build history not found")
	}
	if err != nil {
		r.log.Errorf("query build history entity [%d]: %s", id, err.Error())
		return nil, err
	}
	return entity, nil
}

func (r *BuildHistoryRepo) Create(ctx context.Context, buildID, buildNo uint32, branch string, operatorID uint32, runnerID string) (*ent.BuildHistory, error) {
	builder := r.entClient.Client().BuildHistory.
		Create().
		SetBuildID(buildID).
		SetBuildNo(buildNo).
		SetBranch(branch).
		SetStatus(buildhistory.StatusPending)

	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}
	if runnerID != "" {
		builder.SetRunnerID(runnerID)
	}
	builder.SetUpdatedAt(time.Now())

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create build history [%d/%d]: %s", buildID, buildNo, err.Error())
		return nil, opsV1.ErrorInternalServerError("create build history failed")
	}
	return record, nil
}

// CreateExclusive 独占式创建构建历史:事务内先 FOR UPDATE 锁定构建配置行
// 作为锁锚,再复查未终态历史、推进构建号、落 PENDING 历史,三步对其他触发方
// (手动/API/webhook/cron,乃至其他 core 副本)原子。
//
// 背景:TriggerBuild 原实现是 HasActiveHistory 检查 → IncrBuildNo → Create
// 的 check-then-act,并发触发(两副本同分钟 cron 命中、用户连点、webhook
// 重试)可双双通过检查各跑一条构建。构建配置行是全部触发路径的公共锚,
// 行锁把临界区串行化,等价于"每配置仅允许一条活跃"的互斥,无需引入分布式锁。
// 返回 conflict=true 表示存在未终态历史,调用方回 409。
func (r *BuildHistoryRepo) CreateExclusive(ctx context.Context, buildID uint32, branch string, operatorID uint32, runnerID string) (history *ent.BuildHistory, conflict bool, err error) {
	tx, err := r.entClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf("begin tx for build history [%d]: %s", buildID, err.Error())
		return nil, false, opsV1.ErrorInternalServerError("begin tx failed")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			history = nil
			r.log.Errorf("commit build history [%d]: %s", buildID, err.Error())
			err = opsV1.ErrorInternalServerError("commit build history failed")
		}
	}()

	// 锁锚:配置行 FOR UPDATE,并发触发在此排队
	cfg, err := tx.Build.Query().
		Where(build.IDEQ(buildID)).
		ForUpdate().
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, false, opsV1.ErrorNotFound("build not found")
	}
	if err != nil {
		r.log.Errorf("lock build [%d]: %s", buildID, err.Error())
		return nil, false, opsV1.ErrorInternalServerError("lock build failed")
	}

	exist, err := tx.BuildHistory.Query().
		Where(
			buildhistory.BuildIDEQ(buildID),
			buildhistory.StatusIn(buildhistory.StatusPending, buildhistory.StatusRunning),
		).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query active build history [%d]: %s", buildID, err.Error())
		return nil, false, opsV1.ErrorInternalServerError("check active build failed")
	}
	if exist {
		return nil, true, nil
	}

	buildNo := cfg.BuildNo + 1
	if err = tx.Build.UpdateOneID(buildID).SetBuildNo(buildNo).Exec(ctx); err != nil {
		r.log.Errorf("bump build no [%d]: %s", buildID, err.Error())
		return nil, false, opsV1.ErrorInternalServerError("bump build no failed")
	}

	builder := tx.BuildHistory.Create().
		SetBuildID(buildID).
		SetBuildNo(buildNo).
		SetBranch(branch).
		SetStatus(buildhistory.StatusPending).
		SetUpdatedAt(time.Now())
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}
	if runnerID != "" {
		builder.SetRunnerID(runnerID)
	}
	history, err = builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create build history [%d/%d]: %s", buildID, buildNo, err.Error())
		return nil, false, opsV1.ErrorInternalServerError("create build history failed")
	}
	return history, false, nil
}

// UpdateRunning 置 RUNNING 并记开始时间。
func (r *BuildHistoryRepo) UpdateRunning(ctx context.Context, id uint32) error {
	now := time.Now()
	_, err := r.entClient.Client().BuildHistory.
		UpdateOneID(id).
		SetStatus(buildhistory.StatusRunning).
		SetNillableStartedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
	return err
}

// UpdateFinished 终态回写(状态/日志/产物/失败原因/结束时间)。
func (r *BuildHistoryRepo) UpdateFinished(ctx context.Context, id uint32, status string, logText, errMsg string, bucket, objectKey string, size uint64) error {
	now := time.Now()
	builder := r.entClient.Client().BuildHistory.
		UpdateOneID(id).
		SetStatus(buildhistory.Status(status)).
		SetNillableFinishedAt(&now)

	if logText != "" {
		builder.SetLog(logText)
	}
	if errMsg != "" {
		builder.SetError(errMsg)
	}
	if bucket != "" {
		builder.SetArtifactBucket(bucket)
	}
	if objectKey != "" {
		builder.SetArtifactObjectKey(objectKey)
	}
	if size > 0 {
		builder.SetArtifactSize(size)
	}
	builder.SetUpdatedAt(now)

	_, err := builder.Save(ctx)
	return err
}

// UpdateLogText 增量刷新日志(RUNNING 期间执行器每 5s flush 一次,同时
// 充当孤儿清扫的 runner 心跳)。
func (r *BuildHistoryRepo) UpdateLogText(ctx context.Context, id uint32, logText string) error {
	_, err := r.entClient.Client().BuildHistory.
		UpdateOneID(id).
		SetLog(logText).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	return err
}

// FinalizeInterruptedByRunner 启动清扫:只把本实例(同机器指纹)遗留的
// PENDING/RUNNING 定案 FAILED。构建 goroutine 在进程内,core 重启必然中断
// 本机构建;但多副本部署下全局清扫会把其他副本正在跑的构建也误标 FAILED,
// 故以 runner_id(机器指纹)限定清扫范围。
func (r *BuildHistoryRepo) FinalizeInterruptedByRunner(ctx context.Context, runnerID string) (int, error) {
	now := time.Now()
	q := r.entClient.Client().BuildHistory.
		Update().
		Where(
			buildhistory.StatusIn(buildhistory.StatusPending, buildhistory.StatusRunning),
		)
	if runnerID != "" {
		q = q.Where(buildhistory.RunnerIDEQ(runnerID))
	}
	n, err := q.
		SetStatus(buildhistory.StatusFailed).
		SetError("interrupted by core restart").
		SetNillableFinishedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
	return n, err
}

// buildOrphanStaleThreshold 孤儿构建判定阈值:构建命令最长 3600s 且执行
// 期间每 5s 刷新日志(updated_at 持续更新),克隆(≤10min)与产物上传亦各有
// 时限;超过该阈值仍无任何更新的 PENDING/RUNNING,可判定其 runner 已消失。
// updated_at 由全局 ApplyTimeDefaultHooks 兜底刷新,本仓储逐写点显式
// SetUpdatedAt 是防御性冗余(Create 也写,让 MarkRunning 前卡死的记录
// 同样可被兜底定案)。
const buildOrphanStaleThreshold = 2 * time.Hour

// FinalizeStaleInterrupted 周期清扫孤儿构建:runner 崩溃后无力自 Clean 的
// PENDING/RUNNING 历史(其他实例无权在启动时替它定案),按 updated_at 停更
// 时间兜底定案。条件更新幂等,多副本并发执行恰一方生效。
func (r *BuildHistoryRepo) FinalizeStaleInterrupted(ctx context.Context) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().BuildHistory.
		Update().
		Where(
			buildhistory.StatusIn(buildhistory.StatusPending, buildhistory.StatusRunning),
			buildhistory.UpdatedAtLT(now.Add(-buildOrphanStaleThreshold)),
		).
		SetStatus(buildhistory.StatusFailed).
		SetError("orphaned: runner lost (no progress beyond threshold)").
		SetNillableFinishedAt(&now).
		SetUpdatedAt(now).
		Save(ctx)
	return n, err
}
