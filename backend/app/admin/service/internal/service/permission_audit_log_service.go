package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	auditV1 "go-wind-ops/api/gen/go/audit/service/v1"
	"go-wind-ops/pkg/constants"
)

type PermissionAuditLogService struct {
	adminV1.PermissionAuditLogServiceHTTPServer

	log *log.Helper

	permissionAuditLogServiceClient auditV1.PermissionAuditLogServiceClient
}

func NewPermissionAuditLogService(
	ctx *bootstrap.Context,
	permissionAuditLogServiceClient auditV1.PermissionAuditLogServiceClient,
) *PermissionAuditLogService {
	return &PermissionAuditLogService{
		log:                             ctx.NewLoggerHelper("permission-audit-log/service/admin-service"),
		permissionAuditLogServiceClient: permissionAuditLogServiceClient,
	}
}

func (s *PermissionAuditLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*auditV1.ListPermissionAuditLogResponse, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	resp, err := s.permissionAuditLogServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PermissionAuditLogService) Get(ctx context.Context, req *auditV1.GetPermissionAuditLogRequest) (*auditV1.PermissionAuditLog, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	resp, err := s.permissionAuditLogServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PermissionAuditLogService) Create(ctx context.Context, req *auditV1.CreatePermissionAuditLogRequest) (*emptypb.Empty, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	return s.permissionAuditLogServiceClient.Create(ctx, req)
}
