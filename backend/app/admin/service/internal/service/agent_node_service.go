package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
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


	return s.agentNodeServiceClient.UpdateAgentNode(ctx, req)
}

func (s *AgentNodeService) Delete(ctx context.Context, req *opsV1.DeleteAgentNodeRequest) (*emptypb.Empty, error) {
	_, err := s.agentNodeServiceClient.DeleteAgentNode(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetMetrics 查询节点心跳指标历史(透传 core 时间序列查询)。
func (s *AgentNodeService) GetMetrics(ctx context.Context, req *opsV1.GetNodeMetricsHistoryRequest) (*opsV1.GetNodeMetricsHistoryResponse, error) {
	return s.agentNodeServiceClient.GetNodeMetricsHistory(ctx, req)
}

// Revoke 吊销/恢复节点准入(敏感面:node_manage 权限码点查)。
func (s *AgentNodeService) Revoke(ctx context.Context, req *opsV1.RevokeAgentNodeRequest) (*emptypb.Empty, error) {
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	return s.agentNodeServiceClient.RevokeAgentNode(ctx, req)
}

func (s *AgentNodeService) SendCommand(ctx context.Context, req *opsV1.SendCommandRequest) (*opsV1.SendCommandResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetAction() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: action required")
	}
	if req.GetId() == 0 && req.GetAgentId() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: id or agentId required")
	}

	return s.agentNodeServiceClient.SendCommand(ctx, req)
}

// GetUpgradeUploadUrl 签发 agent 升级包上传预签名 URL(浏览器直传)。
func (s *AgentNodeService) GetUpgradeUploadUrl(ctx context.Context, req *opsV1.GetAgentUpgradeUploadUrlRequest) (*opsV1.GetAgentUpgradeUploadUrlResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	return s.agentNodeServiceClient.GetAgentUpgradeUploadUrl(ctx, req)
}

// Upgrade 远程升级 agent(下发自替换指令)。
func (s *AgentNodeService) Upgrade(ctx context.Context, req *opsV1.UpgradeAgentRequest) (*opsV1.UpgradeAgentResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetNodeId() == 0 || req.GetVersion() == "" || req.GetObjectKey() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: nodeId/version/objectKey required")
	}
	return s.agentNodeServiceClient.UpgradeAgent(ctx, req)
}
