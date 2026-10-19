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

// NotificationChannelService 告警通知渠道门面:透传 core 的 NotificationChannelService。
type NotificationChannelService struct {
	adminV1.NotificationChannelServiceHTTPServer

	log *log.Helper

	notificationChannelServiceClient opsV1.NotificationChannelServiceClient
}

func NewNotificationChannelService(ctx *bootstrap.Context, notificationChannelServiceClient opsV1.NotificationChannelServiceClient) *NotificationChannelService {
	return &NotificationChannelService{
		log:                              ctx.NewLoggerHelper("notification-channel/service/admin-service"),
		notificationChannelServiceClient: notificationChannelServiceClient,
	}
}

func (s *NotificationChannelService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationChannelResponse, error) {
	return s.notificationChannelServiceClient.ListChannels(ctx, req)
}

func (s *NotificationChannelService) Get(ctx context.Context, req *opsV1.GetNotificationChannelRequest) (*opsV1.NotificationChannel, error) {
	return s.notificationChannelServiceClient.GetChannel(ctx, req)
}

func (s *NotificationChannelService) Create(ctx context.Context, req *opsV1.CreateNotificationChannelRequest) (*opsV1.NotificationChannel, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.notificationChannelServiceClient.CreateChannel(ctx, req)
}

func (s *NotificationChannelService) Update(ctx context.Context, req *opsV1.UpdateNotificationChannelRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.notificationChannelServiceClient.UpdateChannel(ctx, req)
}

func (s *NotificationChannelService) Delete(ctx context.Context, req *opsV1.DeleteNotificationChannelRequest) (*emptypb.Empty, error) {
	_, err := s.notificationChannelServiceClient.DeleteChannel(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
