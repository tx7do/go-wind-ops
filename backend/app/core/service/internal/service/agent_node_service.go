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

// AgentNodeService 受管机器(agent 节点)管理服务:gRPC 面向 admin BFF,
// 指令下发经 MQTT 信令通道(AgentService.PublishCommand)。
type AgentNodeService struct {
	opsV1.UnimplementedAgentNodeServiceServer

	log  *log.Helper
	repo *data.AgentNodeRepo

	agentService *AgentService
}

func NewAgentNodeService(ctx *bootstrap.Context, repo *data.AgentNodeRepo, agentService *AgentService) *AgentNodeService {
	return &AgentNodeService{
		log:          ctx.NewLoggerHelper("agent-node/service/core-service"),
		repo:         repo,
		agentService: agentService,
	}
}

func (s *AgentNodeService) ListAgentNodes(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAgentNodeResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *AgentNodeService) GetAgentNode(ctx context.Context, req *opsV1.GetAgentNodeRequest) (*opsV1.AgentNode, error) {
	return s.repo.Get(ctx, req)
}

func (s *AgentNodeService) UpdateAgentNode(ctx context.Context, req *opsV1.UpdateAgentNodeRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	// 仅 name/labels 可更新;其余字段即使误入 mask 也由仓储忽略。
	req.Data.AgentId = nil
	req.Data.Status = nil
	req.Data.Hostname = nil

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AgentNodeService) DeleteAgentNode(ctx context.Context, req *opsV1.DeleteAgentNodeRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SendCommand 向节点下发指令。action 当前支持 ping;载荷为 JSON 字符串,
// 原样透传进 MQTT 指令信封。返回的 request_id 是执行结果回执的幂等键。
func (s *AgentNodeService) SendCommand(ctx context.Context, req *opsV1.SendCommandRequest) (*opsV1.SendCommandResponse, error) {
	if req == nil || req.GetAction() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: action required")
	}
	if req.GetId() == 0 && req.GetAgentId() == "" {
		return nil, opsV1.ErrorBadRequest("invalid parameter: id or agentId required")
	}

	agentId := req.GetAgentId()
	if req.GetId() != 0 {
		node, err := s.repo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: req.Id})
		if err != nil {
			return nil, err
		}
		agentId = node.GetAgentId()
	}

	var payload json.RawMessage
	if raw := req.GetPayload(); raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, opsV1.ErrorBadRequest("payload must be valid JSON")
		}
		payload = json.RawMessage(raw)
	}

	requestId, err := s.agentService.PublishCommand(ctx, agentId, signal.CommandAction(req.GetAction()), payload, s.operatorID(ctx))
	if err != nil {
		s.log.Errorf("dispatch command failed (agent=%s action=%s): %s", agentId, req.GetAction(), err.Error())
		return nil, opsV1.ErrorServiceUnavailable("dispatch command failed: mqtt channel unavailable")
	}

	return &opsV1.SendCommandResponse{RequestId: requestId}, nil
}

// operatorID 从传输层元数据提取操作者(由 admin BFF 的 gRPC 客户端中间件注入)。
func (s *AgentNodeService) operatorID(ctx context.Context) uint32 {
	operator, err := metadata.FromServerContext(ctx)
	if err != nil || operator == nil {
		return 0
	}
	return uint32(operator.GetUserId())
}
