package service

import (
	"net/url"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
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

// SendAlertToChannels 向所有启用的通知渠道(WEBHOOK/EMAIL)发送告警,
// 逐渠道落投递台账(deliveryRepo 非空时)。由监控/预算/证书/节点离线路径调用。
// mwRepo 非空且 suppressible 时:维护窗口激活期间跳过外呼(告警行已落库留痕,
// 不落投递行);恢复通知传 suppressible=false 不受窗口影响。
func SendAlertToChannels(ctx context.Context, repo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, mwRepo *data.MaintenanceWindowRepo, alertID uint32, title, content string, suppressible bool) {
	if repo == nil {
		return
	}
	if suppressible && mwRepo != nil {
		if name := mwRepo.ActiveWindowName(ctx, time.Now()); name != "" {
			log.Infof("alert notification suppressed by maintenance window [%s] (alert=%d title=%s)", name, alertID, title)
			return
		}
	}
	channels, err := repo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, ch := range channels {
		switch ch.Type.String() {
		case "WEBHOOK":
			url := derefStr(ch.WebhookURL)
			if url == "" {
				continue
			}
			sendErr := sendWebhookAlert(url, ch.WebhookSignStyle.String(), derefStr(ch.WebhookSecret), title, content)
			recordDelivery(ctx, deliveryRepo, alertID, ch.ID, "WEBHOOK", maskURL(url), sendErr, title, content)

		case "EMAIL":
			to := derefStr(ch.EmailTo)
			if to == "" || derefStr(ch.SMTPHost) == "" {
				continue
			}
			sendErr := sendEmailAlert(derefStr(ch.SMTPHost), derefInt(ch.SMTPPort),
				derefStr(ch.SMTPUsername), derefStr(ch.SMTPPassword), derefStr(ch.SMTPFrom), to, title, content)
			recordDelivery(ctx, deliveryRepo, alertID, ch.ID, "EMAIL", to, sendErr, title, content)
		}
	}
}

// SendAlertToWebhooks 兼容旧签名:不落投递台账,仅向 WEBHOOK 渠道发送。
func SendAlertToWebhooks(ctx context.Context, repo *data.NotificationChannelRepo, title, content string) {
	channels, err := repo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, ch := range channels {
		if ch.Type.String() != "WEBHOOK" || ch.WebhookURL == nil || *ch.WebhookURL == "" {
			continue
		}
		_ = sendWebhookAlert(*ch.WebhookURL, ch.WebhookSignStyle.String(), derefStr(ch.WebhookSecret), title, content)
	}
}

// maskURL 脱敏目标(去 query 串,钉钉加签 URL 不落库)。
func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func maskURL(u string) string {
	if i := strings.Index(u, "?"); i > 0 {
		return u[:i]
	}
	return u
}

// recordDelivery 落投递台账(失败仅记日志,不影响主流程)。
func recordDelivery(ctx context.Context, deliveryRepo *data.NotificationDeliveryRepo, alertID, channelID uint32, channelType, target string, sendErr error, title, content string) {
	if deliveryRepo == nil || alertID == 0 {
		return
	}
	status, lastError := "SENT", ""
	if sendErr != nil {
		status, lastError = "FAILED", sendErr.Error()
	}
	if err := deliveryRepo.Create(ctx, alertID, channelID, channelType, target, status, lastError, title, content); err != nil {
		fmt.Printf("[delivery] record failed: %v\n", err)
	}
}

// buildWebhookBody 构造各风格群机器人的请求体与最终 URL(纯函数,便于单测):
//   - DINGTALK:文本消息,secret 非空时 URL 追加 timestamp+sign(加签)
//   - FEISHU:文本消息,secret 非空时 body 注入 timestamp+sign(加签)
//   - WECOM:文本消息(无签名机制,secret 忽略)
//   - 其它:通用 JSON(title/content)
func buildWebhookBody(webhookURL, signStyle, secret, title, content string, now time.Time) ([]byte, string) {
	text := title + "\n" + content
	var bodyMap map[string]any
	switch signStyle {
	case "DINGTALK":
		bodyMap = map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
	case "FEISHU":
		bodyMap = map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}
	case "WECOM":
		bodyMap = map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
	default:
		bodyMap = map[string]any{"title": title, "content": content}
	}

	switch {
	case signStyle == "DINGTALK" && secret != "":
		ts := now.UnixMilli()
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(fmt.Sprint(ts) + "\n" + secret))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		sep := "?"
		if strings.Contains(webhookURL, "?") {
			sep = "&"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhookURL, sep, ts, sign)
	case signStyle == "FEISHU" && secret != "":
		ts := now.Unix()
		mac := hmac.New(sha256.New, []byte(fmt.Sprint(ts)+"\n"+secret))
		bodyMap["timestamp"] = fmt.Sprint(ts)
		bodyMap["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return []byte("{}"), webhookURL
	}
	return body, webhookURL
}

// sendWebhookAlert 发送 webhook 告警(支持钉钉/飞书/企微群机器人及其加签)。
func sendWebhookAlert(webhookURL, signStyle, secret, title, content string) error {
	body, finalURL := buildWebhookBody(webhookURL, signStyle, secret, title, content, time.Now())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, finalURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("webhook status %s", resp.Status)
	}
	return nil
}

// sendEmailAlert 经 SMTP 发送告警邮件(587 等端口走 STARTTLS,465 走隐式 TLS)。
func sendEmailAlert(host string, port int, username, password, from, to, title, content string) error {
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	msg := buildMailMessage(from, to, title, content)
	auth := smtp.PlainAuth("", username, password, host)

	if port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		cli, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer cli.Close()
		if err = cli.Auth(auth); err != nil {
			return err
		}
		return sendMailViaClient(cli, from, to, msg)
	}
	return smtp.SendMail(addr, auth, from, splitEmails(to), msg)
}

func sendMailViaClient(cli *smtp.Client, from string, to string, msg []byte) error {
	if err := cli.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range splitEmails(to) {
		if err := cli.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := cli.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return cli.Quit()
}

// buildMailMessage 拼简单 text/plain 邮件(Subject 带 UTF-8 BOM 编码)。
func buildMailMessage(from, to, subject, body string) []byte {
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "From: %s\r\n", from)
	fmt.Fprintf(b, "To: %s\r\n", to)
	fmt.Fprintf(b, "Subject: =?UTF-8?B?%s?=\r\n", base64.StdEncoding.EncodeToString([]byte(subject)))
	fmt.Fprintf(b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(b, "Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	return b.Bytes()
}

func splitEmails(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NotificationDeliveryService 告警通知投递记录查询(只读;写入由发送路径落库)。
type NotificationDeliveryService struct {
	opsV1.UnimplementedNotificationDeliveryServiceServer

	log  *log.Helper
	repo *data.NotificationDeliveryRepo
}

func NewNotificationDeliveryService(ctx *bootstrap.Context, repo *data.NotificationDeliveryRepo) *NotificationDeliveryService {
	return &NotificationDeliveryService{
		log:  ctx.NewLoggerHelper("notification-delivery/service/core-service"),
		repo: repo,
	}
}

func (s *NotificationDeliveryService) ListDeliveries(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListNotificationDeliveryResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *NotificationDeliveryService) GetDelivery(ctx context.Context, req *opsV1.GetNotificationDeliveryRequest) (*opsV1.NotificationDelivery, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}
