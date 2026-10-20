package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	"go-wind-ops/pkg/constants"
)

type LoginAuditLogService struct {
	adminV1.LoginAuditLogServiceHTTPServer

	log *log.Helper

	loginAuditLogServiceClient auditV1.LoginAuditLogServiceClient
}

func NewLoginAuditLogService(ctx *bootstrap.Context, loginAuditLogServiceClient auditV1.LoginAuditLogServiceClient) *LoginAuditLogService {
	return &LoginAuditLogService{
		log:                        ctx.NewLoggerHelper("login-audit-log/service/admin-service"),
		loginAuditLogServiceClient: loginAuditLogServiceClient,
	}
}

func (s *LoginAuditLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*auditV1.ListLoginAuditLogResponse, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.loginAuditLogServiceClient.List(ctx, req)
}

func (s *LoginAuditLogService) Get(ctx context.Context, req *auditV1.GetLoginAuditLogRequest) (*auditV1.LoginAuditLog, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.loginAuditLogServiceClient.Get(ctx, req)
}

func (s *LoginAuditLogService) Create(ctx context.Context, req *auditV1.CreateLoginAuditLogRequest) (*emptypb.Empty, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	return s.loginAuditLogServiceClient.Create(ctx, req)
}
