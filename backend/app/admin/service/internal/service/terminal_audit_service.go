package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/app/admin/service/internal/authzcheck"
	"go-wind-ops/pkg/constants"
	"go-wind-ops/pkg/middleware/auth"
)

// TerminalAuditService 终端会话审计门面:透传 core 的 TerminalService 审计 RPC。
// 终端录制含键盘按键明文,属敏感审计数据——查看要求 sys:audit_logs 权限码
// (authz 中间件引擎当前为 noop,这里显式点查,与 WS 入口同一防线)。
type TerminalAuditService struct {
	adminV1.TerminalAuditServiceHTTPServer

	log *log.Helper

	terminalServiceClient opsV1.TerminalServiceClient

	roleClient permissionV1.RoleServiceClient
	permClient permissionV1.PermissionServiceClient
}

func NewTerminalAuditService(
	ctx *bootstrap.Context,
	terminalServiceClient opsV1.TerminalServiceClient,
	roleClient permissionV1.RoleServiceClient,
	permClient permissionV1.PermissionServiceClient,
) *TerminalAuditService {
	return &TerminalAuditService{
		log:                   ctx.NewLoggerHelper("terminal-audit/service/admin-service"),
		terminalServiceClient: terminalServiceClient,
		roleClient:            roleClient,
		permClient:            permClient,
	}
}

// requireAuditPermission 校验当前操作者持有 sys:audit_logs 权限码。
func (s *TerminalAuditService) requireAuditPermission(ctx context.Context) error {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return err
	}
	allowed, err := authzcheck.HasPermissionCode(ctx, s.roleClient, s.permClient,
		operator.GetUserId(), constants.SystemAuditLogsPermissionCode)
	if err != nil {
		s.log.Errorf("terminal audit permission check failed: %s", err.Error())
		return adminV1.ErrorInternalServerError("authorization check failed")
	}
	if !allowed {
		return adminV1.ErrorForbidden("missing permission " + constants.SystemAuditLogsPermissionCode)
	}
	return nil
}

func (s *TerminalAuditService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListTerminalSessionResponse, error) {
	if err := s.requireAuditPermission(ctx); err != nil {
		return nil, err
	}
	return s.terminalServiceClient.ListTerminalSessions(ctx, req)
}

func (s *TerminalAuditService) Get(ctx context.Context, req *opsV1.GetTerminalSessionRequest) (*opsV1.TerminalSession, error) {
	if err := s.requireAuditPermission(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.terminalServiceClient.GetTerminalSession(ctx, req)
}

func (s *TerminalAuditService) ListChunks(ctx context.Context, req *opsV1.ListTerminalChunksRequest) (*opsV1.ListTerminalChunkResponse, error) {
	if err := s.requireAuditPermission(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetSessionId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: sessionId required")
	}
	return s.terminalServiceClient.ListTerminalChunks(ctx, req)
}
