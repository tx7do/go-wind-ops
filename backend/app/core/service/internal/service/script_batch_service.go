package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// timestamppbOf time.Time → *timestamppb.Timestamp(零值转 nil)。
func timestamppbOf(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// batchItemOutputLimit 批次结果矩阵单条输出截断长度(全量见指令台账)。
const batchItemOutputLimit = 4 << 10

// ScriptBatchService 脚本批量执行:一次把脚本下发到多台节点,逐节点生成
// 指令台账(batch_id 关联);回执由 AgentService 驱动批次计数与终态。
type ScriptBatchService struct {
	opsV1.UnimplementedScriptBatchServiceServer

	log       *log.Helper
	batchRepo *data.ScriptBatchRepo
	recordRepo *data.CommandRecordRepo
	scriptRepo *data.ScriptRepo
	nodeRepo   *data.AgentNodeRepo

	agentService *AgentService
}

func NewScriptBatchService(ctx *bootstrap.Context, batchRepo *data.ScriptBatchRepo, recordRepo *data.CommandRecordRepo, scriptRepo *data.ScriptRepo, nodeRepo *data.AgentNodeRepo, agentService *AgentService) *ScriptBatchService {
	return &ScriptBatchService{
		log:          ctx.NewLoggerHelper("script-batch/service/core-service"),
		batchRepo:    batchRepo,
		recordRepo:   recordRepo,
		scriptRepo:   scriptRepo,
		nodeRepo:     nodeRepo,
		agentService: agentService,
	}
}

func (s *ScriptBatchService) ListScriptBatches(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptBatchResponse, error) {
	return s.batchRepo.List(ctx, req)
}

// GetScriptBatch 批次详情:批次 + 逐节点结果矩阵(从指令台账展开,
// result JSON 解析退出码/时长,输出截断至 4KB)。
func (s *ScriptBatchService) GetScriptBatch(ctx context.Context, req *opsV1.GetScriptBatchRequest) (*opsV1.ScriptBatchDetail, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}

	batch, err := s.batchRepo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	detail := &opsV1.ScriptBatchDetail{Batch: batch}

	records, err := s.recordRepo.ListByBatch(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("query batch items failed")
	}
	for _, rec := range records {
		status := ""
		if rec.Status != nil {
			status = string(*rec.Status)
		}
		item := &opsV1.ScriptBatchItem{
			RequestId: &rec.RequestID,
			NodeId:    rec.NodeID,
			AgentId:   &rec.AgentID,
			Status:    &status,
			Error:     rec.Error,
		}
		if rec.FinishedAt != nil {
			item.FinishedAt = timestamppbOf(*rec.FinishedAt)
		}
		if rec.Result != nil && len(*rec.Result) > 0 {
			var out signal.ScriptExecResult
			if json.Unmarshal([]byte(*rec.Result), &out) == nil {
				exitCode := int32(out.ExitCode)
				item.ExitCode = &exitCode
				item.Duration = &out.Duration
				output := out.Output
				if len(output) > batchItemOutputLimit {
					output = output[:batchItemOutputLimit] + "\n...[truncated]"
				}
				item.Output = &output
			}
		}
		detail.Items = append(detail.Items, item)
	}

	return detail, nil
}

// ExecuteScriptBatch 批量执行:先建批次,再逐节点下发脚本执行指令并把
// 台账挂到批次上;单节点下发失败计入 failed,不影响其余节点。
func (s *ScriptBatchService) ExecuteScriptBatch(ctx context.Context, req *opsV1.ExecuteScriptBatchRequest) (*opsV1.ExecuteScriptBatchResponse, error) {
	if req == nil || req.GetScriptId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: scriptId required")
	}
	if len(req.GetNodeIds()) == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: nodeIds required")
	}

	sc, err := s.scriptRepo.Get(ctx, &opsV1.GetScriptRequest{Id: req.ScriptId})
	if err != nil {
		return nil, err
	}

	timeout := sc.GetTimeoutSeconds()
	if timeout == 0 {
		timeout = signal.ScriptExecTimeoutDefault
	}
	payload := &signal.ScriptExecPayload{
		ScriptId:   sc.GetId(),
		Name:       sc.GetName(),
		Content:    sc.GetContent(),
		TimeoutSec: timeout,
	}
	operatorID := s.operatorID(ctx)

	batch, err := s.batchRepo.Create(ctx, sc.GetId(), sc.GetName(), req.GetNodeIds(),
		uint32(len(req.GetNodeIds())), operatorID)
	if err != nil {
		return nil, opsV1.ErrorInternalServerError("create batch failed")
	}

	var (
		requestIDs []string
		dispatched uint32
		failed     uint32
	)
	for _, nodeID := range req.GetNodeIds() {
		node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
		if err != nil {
			s.log.Warnf("batch [%d] node [%d] not found, skip", batch.ID, nodeID)
			_ = s.batchRepo.ApplyItemResult(ctx, batch.ID, false)
			failed++
			continue
		}

		requestId, err := s.agentService.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
			signal.CommandActionScriptExec, payload, operatorID)
		if err != nil {
			s.log.Errorf("batch [%d] dispatch failed (node=%d): %s", batch.ID, nodeID, err.Error())
			_ = s.batchRepo.ApplyItemResult(ctx, batch.ID, false)
			failed++
			continue
		}

		if err = s.recordRepo.AttachBatch(ctx, requestId, batch.ID); err != nil {
			s.log.Warnf("batch [%d] attach record [%s]: %s", batch.ID, requestId, err.Error())
		}
		requestIDs = append(requestIDs, requestId)
		dispatched++
	}

	s.log.Infof("script batch created (batch=%d script=%d dispatched=%d failed=%d)",
		batch.ID, sc.GetId(), dispatched, failed)
	return &opsV1.ExecuteScriptBatchResponse{
		BatchId:    &batch.ID,
		RequestIds: requestIDs,
		Dispatched: &dispatched,
		Failed:     &failed,
	}, nil
}

func (s *ScriptBatchService) operatorID(ctx context.Context) uint32 {
	operator, err := metadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return 0
	}
	return uint32(operator.GetUserId())
}
