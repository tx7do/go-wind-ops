// maintenance_window_service.go 告警维护窗口 BFF 门面:CRUD 透传 core。
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type MaintenanceWindowService struct {
	adminV1.MaintenanceWindowServiceHTTPServer

	log                            *log.Helper
	maintenanceWindowServiceClient opsV1.MaintenanceWindowServiceClient
}

func NewMaintenanceWindowService(
	ctx *bootstrap.Context,
	maintenanceWindowServiceClient opsV1.MaintenanceWindowServiceClient,
) *MaintenanceWindowService {
	return &MaintenanceWindowService{
		log:                            ctx.NewLoggerHelper("maintenance-window/service/admin-service"),
		maintenanceWindowServiceClient: maintenanceWindowServiceClient,
	}
}

func (s *MaintenanceWindowService) ListMaintenanceWindows(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMaintenanceWindowResponse, error) {
	return s.maintenanceWindowServiceClient.ListMaintenanceWindows(ctx, req)
}

func (s *MaintenanceWindowService) GetMaintenanceWindow(ctx context.Context, req *opsV1.GetMaintenanceWindowRequest) (*opsV1.MaintenanceWindow, error) {
	return s.maintenanceWindowServiceClient.GetMaintenanceWindow(ctx, req)
}

func (s *MaintenanceWindowService) CreateMaintenanceWindow(ctx context.Context, req *opsV1.CreateMaintenanceWindowRequest) (*opsV1.MaintenanceWindow, error) {
	return s.maintenanceWindowServiceClient.CreateMaintenanceWindow(ctx, req)
}

func (s *MaintenanceWindowService) UpdateMaintenanceWindow(ctx context.Context, req *opsV1.UpdateMaintenanceWindowRequest) (*emptypb.Empty, error) {
	return s.maintenanceWindowServiceClient.UpdateMaintenanceWindow(ctx, req)
}

func (s *MaintenanceWindowService) DeleteMaintenanceWindow(ctx context.Context, req *opsV1.DeleteMaintenanceWindowRequest) (*emptypb.Empty, error) {
	return s.maintenanceWindowServiceClient.DeleteMaintenanceWindow(ctx, req)
}
