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

// ScriptService 运维脚本管理门面:透传 core 的 ScriptService。
type ScriptService struct {
	adminV1.ScriptServiceHTTPServer

	log *log.Helper

	scriptServiceClient opsV1.ScriptServiceClient
}

func NewScriptService(ctx *bootstrap.Context, scriptServiceClient opsV1.ScriptServiceClient) *ScriptService {
	return &ScriptService{
		log:                 ctx.NewLoggerHelper("script/service/admin-service"),
		scriptServiceClient: scriptServiceClient,
	}
}

func (s *ScriptService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptResponse, error) {
	return s.scriptServiceClient.ListScripts(ctx, req)
}

func (s *ScriptService) Get(ctx context.Context, req *opsV1.GetScriptRequest) (*opsV1.Script, error) {
	return s.scriptServiceClient.GetScript(ctx, req)
}

func (s *ScriptService) Create(ctx context.Context, req *opsV1.CreateScriptRequest) (*opsV1.Script, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsScriptManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.scriptServiceClient.CreateScript(ctx, req)
}

func (s *ScriptService) Update(ctx context.Context, req *opsV1.UpdateScriptRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsScriptManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.scriptServiceClient.UpdateScript(ctx, req)
}

func (s *ScriptService) Delete(ctx context.Context, req *opsV1.DeleteScriptRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsScriptManagePermissionCode); err != nil {
		return nil, err
	}
	_, err := s.scriptServiceClient.DeleteScript(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ScriptService) Execute(ctx context.Context, req *opsV1.ExecuteScriptRequest) (*opsV1.ExecuteScriptResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsScriptManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.scriptServiceClient.ExecuteScript(ctx, req)
}

// ListScriptVersions 脚本内容版本历史(透传 core)。
func (s *ScriptService) ListScriptVersions(ctx context.Context, req *opsV1.ListScriptVersionsRequest) (*opsV1.ListScriptVersionsResponse, error) {
	return s.scriptServiceClient.ListScriptVersions(ctx, req)
}

// RestoreScriptVersion 恢复到指定版本(透传 core)。
func (s *ScriptService) RestoreScriptVersion(ctx context.Context, req *opsV1.RestoreScriptVersionRequest) (*emptypb.Empty, error) {
	return s.scriptServiceClient.RestoreScriptVersion(ctx, req)
}
