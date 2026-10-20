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

type DataAccessAuditLogService struct {
	adminV1.DataAccessAuditLogServiceHTTPServer

	log *log.Helper

	dataAccessAuditLogServiceClient auditV1.DataAccessAuditLogServiceClient
}

func NewDataAccessAuditLogService(ctx *bootstrap.Context, dataAccessAuditLogServiceClient auditV1.DataAccessAuditLogServiceClient) *DataAccessAuditLogService {
	return &DataAccessAuditLogService{
		log:                             ctx.NewLoggerHelper("data-access-audit-log/service/admin-service"),
		dataAccessAuditLogServiceClient: dataAccessAuditLogServiceClient,
	}
}

func (s *DataAccessAuditLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*auditV1.ListDataAccessAuditLogResponse, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.dataAccessAuditLogServiceClient.List(ctx, req)
}

func (s *DataAccessAuditLogService) Get(ctx context.Context, req *auditV1.GetDataAccessAuditLogRequest) (*auditV1.DataAccessAuditLog, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	return s.dataAccessAuditLogServiceClient.Get(ctx, req)
}

func (s *DataAccessAuditLogService) Create(ctx context.Context, req *auditV1.CreateDataAccessAuditLogRequest) (*emptypb.Empty, error) {

	// 权限码门(敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemAuditLogsPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid parameter")
	}

	return s.dataAccessAuditLogServiceClient.Create(ctx, req)
}
