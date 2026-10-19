package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
)

// PluginService 插件化指令门面:透传 core 的清单查询与远程执行。
type PluginService struct {
	adminV1.PluginServiceHTTPServer

	log *log.Helper

	pluginServiceClient opsV1.PluginServiceClient
}

func NewPluginService(ctx *bootstrap.Context, pluginServiceClient opsV1.PluginServiceClient) *PluginService {
	return &PluginService{
		log:                 ctx.NewLoggerHelper("plugin/service/admin-service"),
		pluginServiceClient: pluginServiceClient,
	}
}

func (s *PluginService) ListAgentPlugins(ctx context.Context, req *opsV1.ListAgentPluginsRequest) (*opsV1.ListAgentPluginsResponse, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetAgentId() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId required")
	}
	return s.pluginServiceClient.ListAgentPlugins(ctx, req)
}

func (s *PluginService) ExecAgentPlugin(ctx context.Context, req *opsV1.ExecAgentPluginRequest) (*opsV1.ExecAgentPluginResponse, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetAgentId() == "" || req.GetAction() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: agentId/action required")
	}
	return s.pluginServiceClient.ExecAgentPlugin(ctx, req)
}
