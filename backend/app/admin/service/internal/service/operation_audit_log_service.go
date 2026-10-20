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

type OperationAuditLogService struct {
	adminV1.OperationAuditLogServiceHTTPServer

	log *log.Helper

	operationAuditLogServiceClient auditV1.OperationAuditLogServiceClient
}

func NewOperationAuditLogService(ctx *bootstrap.Context, operationAuditLogServiceClient auditV1.OperationAuditLogServiceClient) *OperationAuditLogService {
	return &OperationAuditLogService{
		log:                            ctx.NewLoggerHelper("operation-audit-log/service/admin-service"),
		operationAuditLogServiceClient: operationAuditLogServiceClient,
	}
}

func (s *OperationAuditLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*auditV1.ListOperationAuditLogResponse, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.operationAuditLogServiceClient.List(ctx, req)
}

func (s *OperationAuditLogService) Get(ctx context.Context, req *auditV1.GetOperationAuditLogRequest) (*auditV1.OperationAuditLog, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.operationAuditLogServiceClient.Get(ctx, req)
}

func (s *OperationAuditLogService) Create(ctx context.Context, req *auditV1.CreateOperationAuditLogRequest) (*emptypb.Empty, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	return s.operationAuditLogServiceClient.Create(ctx, req)
}
