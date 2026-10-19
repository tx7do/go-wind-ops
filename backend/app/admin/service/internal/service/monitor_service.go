package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// MonitorService 监控规则管理门面:透传 core 的 MonitorService。
type MonitorService struct {
	adminV1.MonitorServiceHTTPServer

	log *log.Helper

	monitorServiceClient opsV1.MonitorServiceClient
}

func NewMonitorService(ctx *bootstrap.Context, monitorServiceClient opsV1.MonitorServiceClient) *MonitorService {
	return &MonitorService{
		log:                  ctx.NewLoggerHelper("monitor/service/admin-service"),
		monitorServiceClient: monitorServiceClient,
	}
}

func (s *MonitorService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMonitorResponse, error) {
	return s.monitorServiceClient.ListMonitors(ctx, req)
}

func (s *MonitorService) Get(ctx context.Context, req *opsV1.GetMonitorRequest) (*opsV1.Monitor, error) {
	return s.monitorServiceClient.GetMonitor(ctx, req)
}

func (s *MonitorService) Create(ctx context.Context, req *opsV1.CreateMonitorRequest) (*opsV1.Monitor, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.monitorServiceClient.CreateMonitor(ctx, req)
}

func (s *MonitorService) Update(ctx context.Context, req *opsV1.UpdateMonitorRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.monitorServiceClient.UpdateMonitor(ctx, req)
}

func (s *MonitorService) Delete(ctx context.Context, req *opsV1.DeleteMonitorRequest) (*emptypb.Empty, error) {
	_, err := s.monitorServiceClient.DeleteMonitor(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
