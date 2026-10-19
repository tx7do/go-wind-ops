package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
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

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemPlatformAdminPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	if err := validateWebhookURL(req.Data.GetWebhookUrl()); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.CreateChannel(ctx, req)
}

func (s *NotificationChannelService) Update(ctx context.Context, req *opsV1.UpdateNotificationChannelRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemPlatformAdminPermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	if err := validateWebhookURL(req.Data.GetWebhookUrl()); err != nil {
		return nil, err
	}
	return s.notificationChannelServiceClient.UpdateChannel(ctx, req)
}

func (s *NotificationChannelService) Delete(ctx context.Context, req *opsV1.DeleteNotificationChannelRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.SystemPlatformAdminPermissionCode); err != nil {
		return nil, err
	}
	_, err := s.notificationChannelServiceClient.DeleteChannel(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// validateWebhookURL 渠道 webhook 地址基础校验:仅 http/https,拒绝环回/
// 链路本地/云元数据地址(告警触发时 core 会向该地址发请求,防 SSRF 探内网;
// 完整出站管控依赖真 authz 引擎与网络层策略)。
func validateWebhookURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return adminV1.ErrorBadRequest("%s", "invalid webhook url")
	}
	host := strings.ToLower(u.Hostname())
	for _, bad := range []string{"localhost", "127.0.0.1", "0.0.0.0", "::1", "169.254.169.254"} {
		if host == bad {
			return adminV1.ErrorBadRequest("%s", "webhook host not allowed")
		}
	}
	return nil
}
