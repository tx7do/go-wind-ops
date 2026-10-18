package service

import (
	"net/url"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// NotificationChannelService 告警通知渠道 CRUD。
type NotificationChannelService struct {
	opsV1.UnimplementedNotificationChannelServiceServer

	log  *log.Helper
	repo *data.NotificationChannelRepo
}

func NewNotificationChannelService(ctx *bootstrap.Context, repo *data.NotificationChannelRepo) *NotificationChannelService {
	return &NotificationChannelService{
		log:  ctx.NewLoggerHelper("notification-channel/service/core-service"),
		repo: repo,
	}
}

func (s *NotificationChannelService) ListChannels(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationChannelResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *NotificationChannelService) GetChannel(ctx context.Context, req *opsV1.GetNotificationChannelRequest) (*opsV1.NotificationChannel, error) {
	return s.repo.Get(ctx, req)
}

func (s *NotificationChannelService) CreateChannel(ctx context.Context, req *opsV1.CreateNotificationChannelRequest) (*opsV1.NotificationChannel, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" {
		return nil, opsV1.ErrorBadRequest("name is required")
	}

	taken, err := s.repo.List(ctx, &paginationV1.PagingRequest{NoPaging: boolPtr(true)})
	if err == nil {
		for _, c := range taken.GetItems() {
			if c.GetName() == req.Data.GetName() {
				return nil, opsV1.ErrorConflict("channel name already exists")
			}
		}
	}

	return s.repo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *NotificationChannelService) UpdateChannel(ctx context.Context, req *opsV1.UpdateNotificationChannelRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *NotificationChannelService) DeleteChannel(ctx context.Context, req *opsV1.DeleteNotificationChannelRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// SendAlertToChannels 向所有启用的 WEBHOOK 渠道发送告警通知。
// 由 CronScheduler 在监控检查产生告警时调用。
func SendAlertToWebhooks(ctx context.Context, repo *data.NotificationChannelRepo, title, content string) {
	channels, err := repo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, ch := range channels {
		if ch.Type.String() != "WEBHOOK" || ch.WebhookURL == nil || *ch.WebhookURL == "" {
			continue
		}
		secret := ""
		if ch.WebhookSecret != nil {
			secret = *ch.WebhookSecret
		}
		go sendWebhookAlert(*ch.WebhookURL, ch.WebhookSignStyle.String(), secret, title, content)
	}
}

// sendWebhookAlert 发送 webhook 告警(支持钉钉/飞书/企微群机器人签名)。
func sendWebhookAlert(webhookURL, signStyle, secret, title, content string) {
	var body []byte
	switch signStyle {
	case "DINGTALK":
		body = []byte(fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s\n%s"}}`, jsonEscape(title), jsonEscape(content)))
	case "FEISHU":
		body = []byte(fmt.Sprintf(`{"msg_type":"text","content":{"text":"%s\n%s"}}`, jsonEscape(title), jsonEscape(content)))
	case "WECOM":
		body = []byte(fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s\n%s"}}`, jsonEscape(title), jsonEscape(content)))
	default:
		body, _ = json.Marshal(map[string]string{"title": title, "content": content})
	}

	// 钉钉加签
	if signStyle == "DINGTALK" && secret != "" {
		ts := fmt.Sprint(time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "\n" + secret))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, ts, sign)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
