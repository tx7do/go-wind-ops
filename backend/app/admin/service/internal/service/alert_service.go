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

// AlertService 告警事件门面:透传 core 的 AlertService。
type AlertService struct {
	adminV1.AlertServiceHTTPServer

	log *log.Helper

	alertServiceClient opsV1.AlertServiceClient
}

func NewAlertService(ctx *bootstrap.Context, alertServiceClient opsV1.AlertServiceClient) *AlertService {
	return &AlertService{
		log:                ctx.NewLoggerHelper("alert/service/admin-service"),
		alertServiceClient: alertServiceClient,
	}
}

func (s *AlertService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAlertResponse, error) {
	return s.alertServiceClient.ListAlerts(ctx, req)
}

func (s *AlertService) Get(ctx context.Context, req *opsV1.GetAlertRequest) (*opsV1.Alert, error) {
	return s.alertServiceClient.GetAlert(ctx, req)
}

func (s *AlertService) Resolve(ctx context.Context, req *opsV1.ResolveAlertRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	_, err := s.alertServiceClient.ResolveAlert(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
