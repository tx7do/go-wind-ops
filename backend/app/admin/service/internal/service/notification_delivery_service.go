package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// NotificationDeliveryService 告警通知投递记录门面:透传 core。
type NotificationDeliveryService struct {
	adminV1.NotificationDeliveryServiceHTTPServer

	log *log.Helper

	notificationDeliveryServiceClient opsV1.NotificationDeliveryServiceClient
}

func NewNotificationDeliveryService(ctx *bootstrap.Context, notificationDeliveryServiceClient opsV1.NotificationDeliveryServiceClient) *NotificationDeliveryService {
	return &NotificationDeliveryService{
		log:                               ctx.NewLoggerHelper("notification-delivery/service/admin-service"),
		notificationDeliveryServiceClient: notificationDeliveryServiceClient,
	}
}

func (s *NotificationDeliveryService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationDeliveryResponse, error) {
	return s.notificationDeliveryServiceClient.ListDeliveries(ctx, req)
}

func (s *NotificationDeliveryService) Get(ctx context.Context, req *opsV1.GetNotificationDeliveryRequest) (*opsV1.NotificationDelivery, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.notificationDeliveryServiceClient.GetDelivery(ctx, req)
}
