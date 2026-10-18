package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/pipelinerun"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// PipelineRunRepo 流水线运行记录仓储(手写转换,单写者模型:
// 执行 goroutine 独占推进,Log/StepResults 增量落库)。
type PipelineRunRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewPipelineRunRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *PipelineRunRepo {
	return &PipelineRunRepo{
		log:       ctx.NewLoggerHelper("pipeline-run/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO ent 实体 → proto。
func (r *PipelineRunRepo) toDTO(e *ent.PipelineRun) (*opsV1.PipelineRun, error) {
	dto := &opsV1.PipelineRun{
		Id:          &e.ID,
		PipelineId:  &e.PipelineID,
		CurrentStep: &e.CurrentStep,
		Error:       e.Error,
		StartedAt:   timeTimestamppb(e.StartedAt),
		FinishedAt:  timeTimestamppb(e.FinishedAt),
		OperatorId:  e.OperatorID,
	}
	if e.Status != nil {
		status := statusToProto(*e.Status)
		dto.Status = &status
	}
	if e.Steps != nil && *e.Steps != "" {
		steps, err := UnmarshalPipelineSteps(*e.Steps)
		if err != nil {
			return nil, err
		}
		dto.Steps = steps
	}
	if e.StepResults != nil && *e.StepResults != "" {
		results, err := UnmarshalPipelineStepResults(*e.StepResults)
		if err != nil {
			return nil, err
		}
		dto.StepResults = results
	}
	return dto, nil
}

// statusToProto ent 枚举 → proto 嵌套枚举。
func statusToProto(v pipelinerun.Status) opsV1.PipelineRun_Status {
	switch v {
	case pipelinerun.StatusRunning:
		return opsV1.PipelineRun_RUNNING
	case pipelinerun.StatusSuccess:
		return opsV1.PipelineRun_SUCCESS
	case pipelinerun.StatusFailed:
		return opsV1.PipelineRun_FAILED
	default:
		return opsV1.PipelineRun_PENDING
	}
}

// Create 触发时建 run(快照步骤 JSON)。
func (r *PipelineRunRepo) Create(ctx context.Context, pipelineID uint32, stepsJSON string, operatorID uint32) (*ent.PipelineRun, error) {
	builder := r.entClient.Client().PipelineRun.Create().
		SetPipelineID(pipelineID).
		SetCurrentStep(-1)
	if stepsJSON != "" {
		builder = builder.SetSteps(stepsJSON)
	}
	if operatorID > 0 {
		builder = builder.SetOperatorID(operatorID)
	}
	return builder.Save(ctx)
}

// MarkRunning 标记执行开始。
func (r *PipelineRunRepo) MarkRunning(ctx context.Context, id uint32) error {
	now := time.Now()
	_, err := r.entClient.Client().PipelineRun.UpdateOneID(id).
		SetStatus(pipelinerun.StatusRunning).
		SetStartedAt(now).
		SetCurrentStep(-1).
		Save(ctx)
	return err
}

// SetCurrentStep 推进当前步骤下标。
func (r *PipelineRunRepo) SetCurrentStep(ctx context.Context, id uint32, idx int32) error {
	_, err := r.entClient.Client().PipelineRun.UpdateOneID(id).SetCurrentStep(idx).Save(ctx)
	return err
}

// AppendLog 追加日志(执行 goroutine 单写者,读改写安全)。
func (r *PipelineRunRepo) AppendLog(ctx context.Context, id uint32, delta string) error {
	if delta == "" {
		return nil
	}
	e, err := r.entClient.Client().PipelineRun.Get(ctx, id)
	if err != nil {
		return err
	}
	text := ""
	if e.Log != nil {
		text = *e.Log
	}
	text += delta
	return r.entClient.Client().PipelineRun.UpdateOneID(id).SetLog(text).Exec(ctx)
}

// SetStepResults 回写步骤结果 JSON。
func (r *PipelineRunRepo) SetStepResults(ctx context.Context, id uint32, resultsJSON string) error {
	builder := r.entClient.Client().PipelineRun.UpdateOneID(id)
	if resultsJSON != "" {
		builder = builder.SetStepResults(resultsJSON)
	}
	_, err := builder.Save(ctx)
	return err
}

// UpdateFinished 终态落库。
func (r *PipelineRunRepo) UpdateFinished(ctx context.Context, id uint32, status pipelinerun.Status, errMsg string) error {
	builder := r.entClient.Client().PipelineRun.UpdateOneID(id).
		SetStatus(status).
		SetFinishedAt(time.Now())
	if errMsg != "" {
		builder = builder.SetError(errMsg)
	}
	_, err := builder.Save(ctx)
	return err
}

// FinalizeInterrupted 启动清扫:上进程崩溃遗留的 RUNNING/PENDING 判 FAILED。
func (r *PipelineRunRepo) FinalizeInterrupted(ctx context.Context) (int, error) {
	runs, err := r.entClient.Client().PipelineRun.Query().
		Where(
			pipelinerun.StatusIn(pipelinerun.StatusRunning, pipelinerun.StatusPending),
		).
		All(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, run := range runs {
		if err = r.UpdateFinished(ctx, run.ID, pipelinerun.StatusFailed, "interrupted by restart"); err == nil {
			count++
		}
	}
	return count, nil
}

// GetEntity 取原始实体(日志 delta 切片用)。
func (r *PipelineRunRepo) GetEntity(ctx context.Context, id uint32) (*ent.PipelineRun, error) {
	return r.entClient.Client().PipelineRun.Get(ctx, id)
}

// Get 详情。
func (r *PipelineRunRepo) Get(ctx context.Context, id uint32) (*opsV1.PipelineRun, error) {
	e, err := r.GetEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e)
}

// List 分页列表(新记录在前;手写分页,steps/results 为 JSON 文本不经 copier)。
func (r *PipelineRunRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineRunResponse, error) {
	client := r.entClient.Client().PipelineRun

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(pipelinerun.FieldID))
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

	resp := &opsV1.ListPipelineRunResponse{Total: uint64(total)}
	for _, e := range rows {
		dto, err := r.toDTO(e)
		if err != nil {
			r.log.Warnf("decode pipeline run (id=%d): %s", e.ID, err.Error())
			continue
		}
		resp.Items = append(resp.Items, dto)
	}
	return resp, nil
}
