package service

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// ScriptService 运维脚本管理:CRUD 落库;Execute 把脚本内容组装进 MQTT
// 指令下发到目标节点,输出/退出码记入指令台账。
type ScriptService struct {
	opsV1.UnimplementedScriptServiceServer

	log      *log.Helper
	repo     *data.ScriptRepo
	nodeRepo *data.AgentNodeRepo

	agentService *AgentService
	approvalRepo *data.ApprovalRepo
}

func NewScriptService(ctx *bootstrap.Context, repo *data.ScriptRepo, nodeRepo *data.AgentNodeRepo, agentService *AgentService, approvalRepo *data.ApprovalRepo) *ScriptService {
	return &ScriptService{
		log:          ctx.NewLoggerHelper("script/service/core-service"),
		repo:         repo,
		nodeRepo:     nodeRepo,
		agentService: agentService,
		approvalRepo: approvalRepo,
	}
}

func (s *ScriptService) ListScripts(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *ScriptService) GetScript(ctx context.Context, req *opsV1.GetScriptRequest) (*opsV1.Script, error) {
	return s.repo.Get(ctx, req)
}

func (s *ScriptService) CreateScript(ctx context.Context, req *opsV1.CreateScriptRequest) (*opsV1.Script, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := validateContent(req.Data.GetContent(), req.Data.GetTimeoutSeconds()); err != nil {
		return nil, err
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("script name already exists")
	}

	return s.repo.Create(ctx, req, s.operatorID(ctx))
}

func (s *ScriptService) UpdateScript(ctx context.Context, req *opsV1.UpdateScriptRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	// 部分更新(如仅改 cron)时 content 不在 mask 内,不参与校验
	if req.Data.Content != nil {
		if err := validateContent(req.Data.GetContent(), req.Data.GetTimeoutSeconds()); err != nil {
			return nil, err
		}
	}

	if req.Data.Name != nil {
		taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("script name already exists")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ScriptService) DeleteScript(ctx context.Context, req *opsV1.DeleteScriptRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ExecuteScript 在目标节点执行脚本:脚本内容进指令载荷(受 256KB 上限约束,
// 超大文件后续走 MinIO),agent 落地临时文件执行;结果见指令台账。
func (s *ScriptService) ExecuteScript(ctx context.Context, req *opsV1.ExecuteScriptRequest) (*opsV1.ExecuteScriptResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id required")
	}
	if req.GetNodeId() == 0 && req.GetAgentId() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: nodeId or agentId required")
	}

	sc, err := s.repo.Get(ctx, &opsV1.GetScriptRequest{Id: &req.Id})
	if err != nil {
		return nil, err
	}

	nodeID := req.GetNodeId()
	agentId := req.GetAgentId()
	if nodeID != 0 {
		node, err := s.nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
		if err != nil {
			return nil, err
		}
		agentId = node.GetAgentId()
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

	// 危险命令确认门槛:命中灾难性模式须显式 confirm_dangerous 才放行。
	// 流水线/定时脚本属自动化调用,由调用点显式置 confirm(脚本入库时作者已审)。
	if ok, patterns := guardDangerousContent(s.log, payload.Content, req.GetConfirmDangerous(), s.operatorID(ctx)); !ok {
		return nil, dangerousCommandError(patterns)
	}

	// 审批闸门:APPROVAL_REQUIRED_ACTIONS 含 script_exec 时先建审批单待批
	if pbJSON, merr := json.Marshal(payload); merr == nil {
		if aerr := approvalGate(ctx, s.log, s.approvalRepo, string(signal.CommandActionScriptExec),
			sc.GetId(), agentId, sc.GetName(), string(pbJSON), s.operatorID(ctx)); aerr != nil {
			return nil, aerr
		}
	}

	requestId, err := s.agentService.PublishCommand(ctx, nodeID, agentId,
		signal.CommandActionScriptExec, payload, s.operatorID(ctx))
	if err != nil {
		s.log.Errorf("execute script failed (script=%d node=%d): %s", sc.GetId(), nodeID, err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	return &opsV1.ExecuteScriptResponse{RequestId: requestId}, nil
}

// validateContent 校验脚本内容大小与超时上限。
func validateContent(content string, timeoutSeconds uint32) error {
	if len(content) == 0 {
		return opsV1.ErrorBadRequest("script content is required")
	}
	if len(content) > signal.ScriptContentLimit {
		return opsV1.ErrorBadRequest("script content exceeds limit (256KB)")
	}
	if timeoutSeconds > signal.ScriptExecTimeoutMax {
		return opsV1.ErrorBadRequest("timeout exceeds limit (600s)")
	}
	return nil
}

func (s *ScriptService) operatorID(ctx context.Context) uint32 {
	operator, err := metadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return 0
	}
	return uint32(operator.GetUserId())
}
