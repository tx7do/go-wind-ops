// pipeline_service.go 构建流水线:配置 CRUD + 触发执行。
// 有序步骤(构建/脚本/分发)按序推进、失败即中止;触发即建运行记录
// 并异步执行,横幅与脚本输出增量落库,前端可按 offset 增量 tail。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/go-utils/trans"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent/commandrecord"
	"go-wind-ops/app/core/service/internal/data/ent/distributionrecord"
	"go-wind-ops/app/core/service/internal/data/ent/pipelinerun"
	appViewer "go-wind-ops/pkg/entgo/viewer"
	"go-wind-ops/pkg/machine"
	pkgMetadata "go-wind-ops/pkg/metadata"

	identityV1 "go-wind-ops/api/gen/go/identity/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const (
	// pipelinePollInterval 步骤终态轮询周期。
	pipelinePollInterval = 3 * time.Second
	// pipelineBuildTimeout 单构建步骤等待上限(构建自身超时上限 1h,另加余量)。
	pipelineBuildTimeout = 70 * time.Minute
	// pipelineScriptTimeout 单脚本步骤等待上限(脚本超时上限 600s,另加余量)。
	pipelineScriptTimeout = 12 * time.Minute
	// pipelineDistTimeout 单分发步骤等待上限。
	pipelineDistTimeout = 20 * time.Minute
	// pipelineStepLogMax 脚本输出进日志的截断上限。
	pipelineStepLogMax = 32 << 10
)

type PipelineService struct {
	opsV1.UnimplementedPipelineServiceServer

	log     *log.Helper
	repo    *data.PipelineRepo
	runRepo *data.PipelineRunRepo

	buildHistoryRepo *data.BuildHistoryRepo
	commandRecordRepo *data.CommandRecordRepo
	distRecordRepo    *data.DistributionRecordRepo

	buildSvc *BuildService
	scriptSvc *ScriptService
	distSvc   *DistributionService

	// runnerID 本实例的执行标识(机器指纹):写入 run 记录,启动清扫与孤儿
	// 清扫据此区分"本实例遗留"与"其他副本在跑"。
	runnerID string
}

func NewPipelineService(
	ctx *bootstrap.Context,
	repo *data.PipelineRepo,
	runRepo *data.PipelineRunRepo,
	buildHistoryRepo *data.BuildHistoryRepo,
	commandRecordRepo *data.CommandRecordRepo,
	distRecordRepo *data.DistributionRecordRepo,
	buildSvc *BuildService,
	scriptSvc *ScriptService,
	distSvc *DistributionService,
) *PipelineService {
	runnerID := machine.MachineID()

	s := &PipelineService{
		log:               ctx.NewLoggerHelper("pipeline/service/core-service"),
		repo:              repo,
		runRepo:           runRepo,
		buildHistoryRepo:  buildHistoryRepo,
		commandRecordRepo: commandRecordRepo,
		distRecordRepo:    distRecordRepo,
		buildSvc:          buildSvc,
		scriptSvc:         scriptSvc,
		distSvc:           distSvc,
		runnerID:          runnerID,
	}

	// 启动清扫:本实例(同机器指纹)上次重启中断的 run 定案 FAILED。
	// 其他实例的未终态 run 归调度器的孤儿清扫(超阈值兜底)。
	if n, err := runRepo.FinalizeInterruptedByRunner(ctx.Context(), runnerID); err == nil && n > 0 {
		s.log.Infof("finalized %d interrupted pipeline run(s) (runner=%s)", n, runnerID)
	}

	return s
}

// ── 配置 CRUD ──

func (s *PipelineService) ListPipelines(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *PipelineService) GetPipeline(ctx context.Context, req *opsV1.GetPipelineRequest) (*opsV1.Pipeline, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}

func (s *PipelineService) CreatePipeline(ctx context.Context, req *opsV1.CreatePipelineRequest) (*opsV1.Pipeline, error) {
	if req == nil || req.Data == nil || req.Data.GetName() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: name required")
	}
	if err := s.validateSteps(req.Data.GetSteps()); err != nil {
		return nil, err
	}
	if taken, _ := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0); taken {
		return nil, opsV1.ErrorConflict("pipeline name already exists")
	}
	stepsJSON, err := data.MarshalPipelineSteps(req.Data.GetSteps())
	if err != nil {
		return nil, opsV1.ErrorBadRequest("%s", err.Error())
	}
	dto, err := s.repo.Create(ctx, req.Data.GetName(), stepsJSON, req.Data.Description, req.Data.GetLabels(), operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("create pipeline failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create failed")
	}
	return dto, nil
}

func (s *PipelineService) UpdatePipeline(ctx context.Context, req *opsV1.UpdatePipelineRequest) (*emptypb.Empty, error) {
	return s.update(ctx, req)
}

func (s *PipelineService) update(ctx context.Context, req *opsV1.UpdatePipelineRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.validateSteps(req.Data.GetSteps()); err != nil {
		return nil, err
	}
	if req.Data.GetName() != "" {
		if taken, _ := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId()); taken {
			return nil, opsV1.ErrorConflict("pipeline name already exists")
		}
	}
	stepsJSON, err := data.MarshalPipelineSteps(req.Data.GetSteps())
	if err != nil {
		return nil, opsV1.ErrorBadRequest("%s", err.Error())
	}
	if _, err = s.repo.Update(ctx, req.GetId(), req.Data.GetName(), stepsJSON, req.Data.Description, req.Data.GetLabels()); err != nil {
		s.log.Errorf("update pipeline [%d] failed: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("update failed")
	}
	return &emptypb.Empty{}, nil
}

func (s *PipelineService) DeletePipeline(ctx context.Context, req *opsV1.DeletePipelineRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

// validateSteps 步骤合法性:非空、ref_id 必填、SCRIPT 步骤需目标机器。
func (s *PipelineService) validateSteps(steps []*opsV1.PipelineStep) error {
	if len(steps) == 0 {
		return opsV1.ErrorBadRequest("pipeline requires at least one step")
	}
	for i, st := range steps {
		if st.GetType() == opsV1.PipelineStep_STEP_TYPE_UNSPECIFIED || st.GetRefId() == 0 {
			return opsV1.ErrorBadRequest("step #%d: type/refId required", i+1)
		}
		if st.GetType() == opsV1.PipelineStep_SCRIPT && st.GetAgentId() == "" {
			return opsV1.ErrorBadRequest("step #%d: script step requires agentId", i+1)
		}
	}
	return nil
}

// ── 触发与执行 ──

// TriggerPipeline 创建运行记录并异步执行。
func (s *PipelineService) TriggerPipeline(ctx context.Context, req *opsV1.TriggerPipelineRequest) (*opsV1.TriggerPipelineResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	pipeline, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("pipeline not found")
	}
	stepsJSON, err := data.MarshalPipelineSteps(pipeline.GetSteps())
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("%s", err.Error())
	}
	operatorID := operatorIDOf(ctx)
	// 租户归因:执行 goroutine 脱离请求生命周期,显式带上触发者的租户,
	// 步骤指令台账/脚本执行数据才能落对租户(否则落 tenant 0)。
	tid := viewerTenantOf(ctx)
	run, err := s.runRepo.Create(ctx, pipeline.GetId(), stepsJSON, operatorID, s.runnerID)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("create run failed")
	}

	go s.executeRun(run.ID, pipeline.GetSteps(), operatorID, tid)

	s.log.Infof("pipeline triggered (pipeline=%d run=%d steps=%d operator=%d tenant=%d)",
		pipeline.GetId(), run.ID, len(pipeline.GetSteps()), operatorID, tid)
	return &opsV1.TriggerPipelineResponse{RunId: &run.ID}, nil
}

// pipelineDetachedCtx 执行 goroutine 的上下文:脱离请求生命周期,保留操作人
// 与租户。注意 metadata 只供 operatorIDOf 归因读取,viewer 必须显式注入——
// 直连调用不经 ent.Server 中间件,没有 viewer 会兜底 SystemViewer(tenant 0)。
func pipelineDetachedCtx(operatorID, tenantID uint32) context.Context {
	c, _ := pkgMetadata.NewContext(context.Background(),
		pkgMetadata.NewUserOperator(uint64(operatorID), uint64(tenantID), 0, identityV1.DataScope_ALL))
	if tenantID > 0 {
		c = viewer.WithContext(c, appViewer.NewUserViewer(uint64(operatorID), uint64(tenantID), 0, "", identityV1.DataScope_ALL))
	}
	return c
}

// executeRun 按序推进步骤,失败即中止;结果与日志增量落库。
func (s *PipelineService) executeRun(runID uint32, steps []*opsV1.PipelineStep, operatorID, tenantID uint32) {
	ctx := pipelineDetachedCtx(operatorID, tenantID)

	if err := s.runRepo.MarkRunning(ctx, runID); err != nil {
		s.log.Errorf("pipeline run mark running failed (run=%d): %s", runID, err.Error())
		return
	}
	s.appendLogf(ctx, runID, "⏵ pipeline run #%d started (%d steps)\n", runID, len(steps))

	results := make([]*opsV1.PipelineStepResult, 0, len(steps))
	flushResults := func() {
		if jsonText, err := data.MarshalPipelineStepResults(results); err == nil {
			_ = s.runRepo.SetStepResults(ctx, runID, jsonText)
		}
	}

	var failErr error
	for i, st := range steps {
		_ = s.runRepo.SetCurrentStep(ctx, runID, int32(i))

		typeName := st.GetType().String()
		display := stepDisplayName(st)
		s.appendLogf(ctx, runID, "▶ step %d/%d [%s] %s (ref=%d)\n", i+1, len(steps), typeName, display, st.GetRefId())

		start := time.Now()
		var stepErr error
		switch st.GetType() {
		case opsV1.PipelineStep_BUILD:
			stepErr = s.runBuildStep(ctx, runID, st)
		case opsV1.PipelineStep_SCRIPT:
			stepErr = s.runScriptStep(ctx, runID, st)
		case opsV1.PipelineStep_DISTRIBUTION:
			stepErr = s.runDistributionStep(ctx, runID, st)
		default:
			stepErr = fmt.Errorf("unknown step type")
		}
		durationMs := time.Since(start).Milliseconds()

		result := &opsV1.PipelineStepResult{
			Type:      &typeName,
			RefId:     st.RefId,
			Name:      &display,
			DurationMs: &durationMs,
		}
		if stepErr != nil {
			status := "FAILED"
			msg := stepErr.Error()
			result.Status = &status
			result.Error = &msg
			results = append(results, result)
			flushResults()
			s.appendLogf(ctx, runID, "✗ step %d/%d failed (%s): %s\n", i+1, len(steps), time.Duration(durationMs)*time.Millisecond, msg)
			failErr = fmt.Errorf("step %d [%s] failed: %s", i+1, typeName, msg)
			break
		}
		status := "SUCCESS"
		result.Status = &status
		results = append(results, result)
		flushResults()
		s.appendLogf(ctx, runID, "✓ step %d/%d done (%s)\n", i+1, len(steps), time.Duration(durationMs)*time.Millisecond)
	}

	// 未执行到的步骤标 SKIPPED
	for j := len(results); j < len(steps); j++ {
		status := "SKIPPED"
		name := stepDisplayName(steps[j])
		typeName := steps[j].GetType().String()
		results = append(results, &opsV1.PipelineStepResult{
			Type: &typeName, RefId: steps[j].RefId, Name: &name, Status: &status,
		})
	}
	flushResults()

	if failErr != nil {
		_ = s.runRepo.UpdateFinished(ctx, runID, pipelinerun.StatusFailed, failErr.Error())
		s.appendLogf(ctx, runID, "⏹ pipeline run #%d FAILED: %s\n", runID, failErr.Error())
		s.log.Warnf("pipeline run failed (run=%d): %s", runID, failErr.Error())
		return
	}
	_ = s.runRepo.UpdateFinished(ctx, runID, pipelinerun.StatusSuccess, "")
	s.appendLogf(ctx, runID, "⏹ pipeline run #%d SUCCESS\n", runID)
	s.log.Infof("pipeline run success (run=%d)", runID)
}

func stepDisplayName(st *opsV1.PipelineStep) string {
	if st.GetName() != "" {
		return st.GetName()
	}
	return fmt.Sprintf("%s#%d", st.GetType().String(), st.GetRefId())
}

// runBuildStep 构建:触发后轮询构建历史终态。
func (s *PipelineService) runBuildStep(ctx context.Context, runID uint32, st *opsV1.PipelineStep) error {
	resp, err := s.buildSvc.TriggerBuild(ctx, &opsV1.TriggerBuildRequest{Id: st.GetRefId()})
	if err != nil {
		return fmt.Errorf("trigger build: %s", err.Error())
	}
	s.appendLogf(ctx, runID, "  build #%d triggered (history=%d)\n", resp.GetBuildNo(), resp.GetHistoryId())

	historyID := resp.GetHistoryId()
	err = s.awaitCondition(ctx, pipelineBuildTimeout, func(ctx context.Context) (bool, error) {
		h, err := s.buildHistoryRepo.Get(ctx, &opsV1.GetBuildHistoryRequest{Id: &historyID})
		if err != nil {
			return false, err
		}
		switch h.GetStatus() {
		case opsV1.BuildHistory_SUCCESS:
			return true, nil
		case opsV1.BuildHistory_FAILED:
			return false, fmt.Errorf("build history [%d] failed", historyID)
		default:
			return false, nil
		}
	})
	if err != nil {
		return err
	}
	return nil
}

// runScriptStep 脚本:同步下发后按指令台账轮询终态,输出进日志。
func (s *PipelineService) runScriptStep(ctx context.Context, runID uint32, st *opsV1.PipelineStep) error {
		// 自动化链路:脚本内容在流水线编排/脚本入库时已被作者审阅,此处显式确认通过危险命令门槛。
	resp, err := s.scriptSvc.ExecuteScript(ctx, &opsV1.ExecuteScriptRequest{Id: st.GetRefId(), AgentId: st.AgentId, ConfirmDangerous: trans.Ptr(true)})
	if err != nil {
		return fmt.Errorf("execute script: %s", err.Error())
	}
	s.appendLogf(ctx, runID, "  script dispatched to %s (request=%s)\n", st.GetAgentId(), resp.GetRequestId())

	err = s.awaitCondition(ctx, pipelineScriptTimeout, func(ctx context.Context) (bool, error) {
		rec, err := s.commandRecordRepo.GetByRequestID(ctx, resp.GetRequestId())
		if err != nil {
			return false, nil // 台账短暂不可见(写入中)时继续等
		}
		switch derefStatus(rec.Status) {
		case commandrecord.StatusSuccess:
			return true, nil
		case commandrecord.StatusFailed:
			if rec.Error != nil && *rec.Error != "" {
				return false, fmt.Errorf("script failed: %s", *rec.Error)
			}
			return false, fmt.Errorf("script failed (request=%s)", resp.GetRequestId())
		default:
			return false, nil
		}
	})
	if err != nil {
		// 失败时尽量把脚本输出带回日志
		if rec, gerr := s.commandRecordRepo.GetByRequestID(ctx, resp.GetRequestId()); gerr == nil && rec.Result != nil {
			s.appendScriptOutput(ctx, runID, *rec.Result)
		}
		return err
	}

	if rec, gerr := s.commandRecordRepo.GetByRequestID(ctx, resp.GetRequestId()); gerr == nil && rec.Result != nil {
		s.appendScriptOutput(ctx, runID, *rec.Result)
	}
	return nil
}

// runDistributionStep 分发:触发后轮询全部目标记录终态(任一 FAILED 即失败)。
func (s *PipelineService) runDistributionStep(ctx context.Context, runID uint32, st *opsV1.PipelineStep) error {
	resp, err := s.distSvc.TriggerDistribution(ctx, &opsV1.TriggerDistributionRequest{Id: st.GetRefId()})
	if err != nil {
		return fmt.Errorf("trigger distribution: %s", err.Error())
	}
	ids := resp.GetRecordIds()
	if len(ids) == 0 && resp.GetRecordId() != 0 {
		ids = []uint32{resp.GetRecordId()}
	}
	s.appendLogf(ctx, runID, "  distribution triggered (%d targets)\n", len(ids))

	var failMsg string
	err = s.awaitCondition(ctx, pipelineDistTimeout, func(ctx context.Context) (bool, error) {
		pending := 0
		failMsg = ""
		for _, id := range ids {
			rec, rerr := s.distRecordRepo.GetEntity(ctx, id)
			if rerr != nil {
				continue
			}
			switch derefDistStatus(rec.Status) {
			case distributionrecord.StatusSuccess:
			case distributionrecord.StatusFailed:
				if rec.Error != nil {
					failMsg = fmt.Sprintf("distribution record [%d] failed: %s", id, *rec.Error)
				} else {
					failMsg = fmt.Sprintf("distribution record [%d] failed", id)
				}
			case distributionrecord.StatusSkipped:
				failMsg = fmt.Sprintf("distribution record [%d] skipped (earlier batch failed)", id)
			default:
				pending++
			}
		}
		if failMsg != "" {
			return false, fmt.Errorf("%s", failMsg)
		}
		return pending == 0, nil
	})
	return err
}

// awaitCondition 周期轮询至条件成立/失败/超时。
func (s *PipelineService) awaitCondition(ctx context.Context, timeout time.Duration, cond func(context.Context) (bool, error)) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(pipelinePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout after %s", timeout)
		case <-ticker.C:
			done, err := cond(ctx)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		}
	}
}

// appendScriptOutput 解析脚本回执结果,把输出(截断)写进运行日志。
func (s *PipelineService) appendScriptOutput(ctx context.Context, runID uint32, resultJSON string) {
	var out struct {
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &out); err != nil || out.Output == "" {
		return
	}
	output := out.Output
	if len(output) > pipelineStepLogMax {
		output = output[:pipelineStepLogMax] + "\n...(truncated)"
	}
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	s.appendLogf(ctx, runID, "  script output (exit=%d):\n%s", out.ExitCode, output)
}

// appendLogf 追加日志(执行 goroutine 单写者)。
func (s *PipelineService) appendLogf(ctx context.Context, runID uint32, format string, args ...any) {
	_ = s.runRepo.AppendLog(ctx, runID, fmt.Sprintf(format, args...))
}

// ── 运行记录查询 ──

func (s *PipelineService) ListPipelineRuns(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineRunResponse, error) {
	return s.runRepo.List(ctx, req)
}

func (s *PipelineService) GetPipelineRun(ctx context.Context, req *opsV1.GetPipelineRunRequest) (*opsV1.PipelineRun, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.runRepo.Get(ctx, req.GetId())
}

// GetPipelineRunLog 增量返回运行日志(与构建日志同一切分约定)。
func (s *PipelineService) GetPipelineRunLog(ctx context.Context, req *opsV1.GetPipelineRunLogRequest) (*opsV1.GetPipelineRunLogResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	entity, err := s.runRepo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("pipeline run not found")
	}

	logText := ""
	if entity.Log != nil {
		logText = *entity.Log
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(logText)) {
		offset = 0
	}

	delta := logText[offset:]
	newOffset := int64(len(logText))
	status := ""
	if entity.Status != nil {
		status = string(*entity.Status)
	}
	finished := status == "SUCCESS" || status == "FAILED"

	return &opsV1.GetPipelineRunLogResponse{
		Delta:     &delta,
		NewOffset: &newOffset,
		Status:    &status,
		Finished:  &finished,
	}, nil
}

// derefStatus 指针枚举安全解引用(空值视为 PENDING)。
func derefStatus(v *commandrecord.Status) commandrecord.Status {
	if v == nil {
		return commandrecord.StatusPending
	}
	return *v
}

// derefDistStatus 指针枚举安全解引用(空值视为 PENDING)。
func derefDistStatus(v *distributionrecord.Status) distributionrecord.Status {
	if v == nil {
		return distributionrecord.StatusPending
	}
	return *v
}
