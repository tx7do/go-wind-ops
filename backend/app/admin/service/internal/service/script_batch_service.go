package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
)

// ScriptBatchService 脚本批量执行门面:透传 core 的 ScriptBatchService。
type ScriptBatchService struct {
	adminV1.ScriptBatchServiceHTTPServer

	log *log.Helper

	scriptBatchServiceClient opsV1.ScriptBatchServiceClient
}

func NewScriptBatchService(ctx *bootstrap.Context, scriptBatchServiceClient opsV1.ScriptBatchServiceClient) *ScriptBatchService {
	return &ScriptBatchService{
		log:                      ctx.NewLoggerHelper("script-batch/service/admin-service"),
		scriptBatchServiceClient: scriptBatchServiceClient,
	}
}

func (s *ScriptBatchService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptBatchResponse, error) {
	return s.scriptBatchServiceClient.ListScriptBatches(ctx, req)
}

func (s *ScriptBatchService) Get(ctx context.Context, req *opsV1.GetScriptBatchRequest) (*opsV1.ScriptBatchDetail, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.scriptBatchServiceClient.GetScriptBatch(ctx, req)
}

func (s *ScriptBatchService) BatchExec(ctx context.Context, req *opsV1.ExecuteScriptBatchRequest) (*opsV1.ExecuteScriptBatchResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsScriptManagePermissionCode); err != nil {
		return nil, err
	}
	// 即席命令模式(command+agent_ids)与脚本模式(script_id+node_ids)二选一
	valid := req != nil && (
		(req.GetCommand() != "" && (len(req.GetAgentIds()) > 0 || len(req.GetNodeIds()) > 0)) ||
			(req.GetScriptId() != 0 && len(req.GetNodeIds()) > 0))
	if !valid {
		return nil, adminV1.ErrorBadRequest("invalid request: command+agentIds or scriptId+nodeIds required")
	}
	return s.scriptBatchServiceClient.ExecuteScriptBatch(ctx, req)
}
