package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// AgentNodeService 受管机器(agent 节点)管理门面:透传 core 的
// AgentNodeService(节点数据与 MQTT 指令通道都在 core)。
type AgentNodeService struct {
	adminV1.AgentNodeServiceHTTPServer

	log *log.Helper

	agentNodeServiceClient opsV1.AgentNodeServiceClient
}

func NewAgentNodeService(ctx *bootstrap.Context, agentNodeServiceClient opsV1.AgentNodeServiceClient) *AgentNodeService {
	return &AgentNodeService{
		log:                    ctx.NewLoggerHelper("agent-node/service/admin-service"),
		agentNodeServiceClient: agentNodeServiceClient,
	}
}

func (s *AgentNodeService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAgentNodeResponse, error) {
	return s.agentNodeServiceClient.ListAgentNodes(ctx, req)
}

func (s *AgentNodeService) Get(ctx context.Context, req *opsV1.GetAgentNodeRequest) (*opsV1.AgentNode, error) {
	return s.agentNodeServiceClient.GetAgentNode(ctx, req)
}

func (s *AgentNodeService) Update(ctx context.Context, req *opsV1.UpdateAgentNodeRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.agentNodeServiceClient.UpdateAgentNode(ctx, req)
}

func (s *AgentNodeService) Delete(ctx context.Context, req *opsV1.DeleteAgentNodeRequest) (*emptypb.Empty, error) {
	_, err := s.agentNodeServiceClient.DeleteAgentNode(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *AgentNodeService) SendCommand(ctx context.Context, req *opsV1.SendCommandRequest) (*opsV1.SendCommandResponse, error) {
	if req == nil || req.GetAction() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: action required")
	}
	if req.GetId() == 0 && req.GetAgentId() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: id or agentId required")
	}

	return s.agentNodeServiceClient.SendCommand(ctx, req)
}
