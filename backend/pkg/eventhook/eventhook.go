// Package eventhook 平台事件外发:把审批建单/分发失败/晋升触发等平台事件
// POST 到企业自有系统(EVENT_WEBHOOK_URL,空=关闭),区别于告警渠道——告警
// 是"运维异常的运维通知",这里是"平台状态变化的系统集成"。
//
// 语义:阻塞失败不重试(集成方拉不到再靠指令台账/审批页对账),5s 超时,
// 独立 goroutine 派发不阻塞主流程;payload 统一信封 {event, time, data};
// 可选 EVENT_WEBHOOK_SECRET 时按 X-Signature: sha256=<HMAC-SHA256(body)>
// 签名(与 git webhook 同规,接收方零新概念)。
package eventhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"fmt"
	"os"
	"time"
)

const timeout = 5 * time.Second

// Envelope 事件信封。
type Envelope struct {
	Event string          `json:"event"`
	Time  string          `json:"time"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func urlAndSecret() (string, string) {
	return os.Getenv("EVENT_WEBHOOK_URL"), os.Getenv("EVENT_WEBHOOK_SECRET")
}

// Enabled 是否配置了外发端点。
func Enabled() bool {
	u, _ := urlAndSecret()
	return u != ""
}

// Emit 异步外发事件(不阻塞调用方、不返回错误——失败仅记 stderr,集成方
// 以平台内数据为准)。data 为任意可 JSON 序列化的事件载荷。
func Emit(event string, data any) {
	url, secret := urlAndSecret()
	if url == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event": event,
		"time":  time.Now().UTC().Format(time.RFC3339),
		"data":  data,
	})
	if err != nil {
		return
	}
	go post(url, secret, body)
}

// EmitSync 同步外发(调用方需要确保送达顺序的场景,仍有 5s 上限)。
func EmitSync(ctx context.Context, event string, data any) {
	url, secret := urlAndSecret()
	if url == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event": event,
		"time":  time.Now().UTC().Format(time.RFC3339),
		"data":  data,
	})
	if err != nil {
		return
	}
	_ = postCtx(ctx, url, secret, body)
}

func post(url, secret string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = postCtx(ctx, url, secret, body)
}

func postCtx(ctx context.Context, url, secret string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "eventhook %s: %v\n", eventName(body), err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		_, _ = fmt.Fprintf(os.Stderr, "eventhook %s: status %s\n", eventName(body), resp.Status)
	}
	return nil
}

// eventName 从信封里取事件名(仅日志用)。
func eventName(body []byte) string {
	var e Envelope
	if json.Unmarshal(body, &e) == nil {
		return e.Event
	}
	return "?"
}
