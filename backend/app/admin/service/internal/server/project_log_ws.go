// project_log_ws.go 项目日志实时 tail WebSocket 端点:
// 浏览器 WS ↔ 本服务(参数 token 鉴权) ↔ core GetProjectLog(增量 offset) ↔ MQTT ↔ agent。
//
// 服务端按查询参数 interval 周期拉取增量,新字节以二进制帧推送;
// 检测到日志重置(轮转/清空)时发文本帧 {"event":"RESET"},浏览器清空缓冲。
//
// 本路由经 kratos Server.Handle 直接注册在 mux 上,不经过鉴权中间件,
// 鉴权完全由查询参数 token 完成。
package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/pkg/middleware/auth"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const projectLogWsPath = "/admin/v1/ws/projects/log"

// ProjectLogWsHandler 项目日志实时 tail WS 处理器。
type ProjectLogWsHandler struct {
	checker auth.AccessTokenChecker
	client  opsV1.ProjectServiceClient

	upgrader websocket.Upgrader
}

// NewProjectLogWsHandler 构造处理器。
func NewProjectLogWsHandler(ctx *bootstrap.Context, checker auth.AccessTokenChecker, client opsV1.ProjectServiceClient) *ProjectLogWsHandler {
	_ = ctx
	return &ProjectLogWsHandler{
		checker: checker,
		client:  client,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Path 注册路径(rest_server.go Handle 用)。
func (h *ProjectLogWsHandler) Path() string { return projectLogWsPath }

func (h *ProjectLogWsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" || h.checker == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if valid, _ := h.checker.IsValidAccessToken(r.Context(), token, false); !valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	projectID, perr := strconv.ParseUint(r.URL.Query().Get("projectId"), 10, 32)
	if perr != nil || projectID == 0 {
		http.Error(w, "projectId required", http.StatusBadRequest)
		return
	}

	intervalMs := 2000
	if v,VERR := strconv.Atoi(r.URL.Query().Get("interval")); VERR == nil && v >= 1000 && v <= 10000 {
		intervalMs = v
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"OPENED"}`))

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 客户端断开即停止拉取循环
	go func() {
		for {
			if _, _, rerr := conn.ReadMessage(); rerr != nil {
				cancel()
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		resp, err := h.client.GetProjectLog(ctx, &opsV1.GetProjectLogRequest{
			Id:     uint32Ptr(uint32(projectID)),
			Offset: int64Ptr(offset),
		})
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage,
				[]byte(`{"event":"ERROR","error":`+wsQuote(err.Error())+`}`))
			continue
		}

		// 重置:日志被清空/轮转,通知浏览器清空缓冲后按当前内容续传
		if resp.GetTruncated() && offset > 0 {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"RESET"}`))
		}
		offset = resp.GetOffset()

		if len(resp.GetContentB64()) > 0 {
			if werr := conn.WriteMessage(websocket.BinaryMessage, resp.GetContentB64()); werr != nil {
				return
			}
		}
	}
}

func uint32Ptr(v uint32) *uint32 { return &v }
func int64Ptr(v int64) *int64 { return &v }
