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

func (r *BuildHistoryRepo) Create(ctx context.Context, buildID, buildNo uint32, branch string, operatorID uint32) (*ent.BuildHistory, error) {
	builder := r.entClient.Client().BuildHistory.
		Create().
		SetBuildID(buildID).
		SetBuildNo(buildNo).
		SetBranch(branch).
		SetStatus(buildhistory.StatusPending)

	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create build history [%d/%d]: %s", buildID, buildNo, err.Error())
		return nil, opsV1.ErrorInternalServerError("create build history failed")
	}
	return record, nil
}

// HasActiveHistory 是否存在未终态的构建(PENDING/RUNNING),用于触发去重。
func (r *BuildHistoryRepo) HasActiveHistory(ctx context.Context, buildID uint32) (bool, error) {
	exist, err := r.entClient.Client().BuildHistory.
		Query().
		Where(
			buildhistory.BuildIDEQ(buildID),
			buildhistory.StatusIn(buildhistory.StatusPending, buildhistory.StatusRunning),
		).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query active build history: %s", err.Error())
		return false, err
	}
	return exist, nil
}

// UpdateRunning 置 RUNNING 并记开始时间。
func (r *BuildHistoryRepo) UpdateRunning(ctx context.Context, id uint32) error {
	now := time.Now()
	_, err := r.entClient.Client().BuildHistory.
		UpdateOneID(id).
		SetStatus(buildhistory.StatusRunning).
		SetNillableStartedAt(&now).
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

	_, err := builder.Save(ctx)
	return err
}

// UpdateLogText 增量刷新日志(RUNNING 期间调用,让前端轮询可见进度)。
func (r *BuildHistoryRepo) UpdateLogText(ctx context.Context, id uint32, logText string) error {
	_, err := r.entClient.Client().BuildHistory.
		UpdateOneID(id).
		SetLog(logText).
		Save(ctx)
	return err
}

// FinalizeInterrupted 服务启动时把上次遗留的 PENDING/RUNNING 定案 FAILED
// (core 重启会中断进行中的构建),返回行数。
func (r *BuildHistoryRepo) FinalizeInterrupted(ctx context.Context) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().BuildHistory.
		Update().
		Where(
			buildhistory.StatusIn(buildhistory.StatusPending, buildhistory.StatusRunning),
		).
		SetStatus(buildhistory.StatusFailed).
		SetError("interrupted by core restart").
		SetNillableFinishedAt(&now).
		Save(ctx)
	return n, err
}
