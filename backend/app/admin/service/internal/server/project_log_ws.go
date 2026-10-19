// project_log_ws.go 项目日志实时 tail WebSocket 端点:
// 浏览器 WS ↔ 本服务(一次性票据握手) ↔ core GetProjectLog(增量 offset) ↔ MQTT ↔ agent。
//
// 服务端按查询参数 interval 周期拉取增量,新字节以二进制帧推送;
// 检测到日志重置(轮转/清空)时发文本帧 {"event":"RESET"},浏览器清空缓冲。
//
// 握手用 30 秒一次性票据(kind=project-log,签发期校验 ops:project_manage
// 并绑定 projectId),GETDEL 原子消费;操作者身份以出站元数据头携带,
// core 侧据此构建租户 viewer,防跨租户读日志。
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	grpcmd "google.golang.org/grpc/metadata"

	appMetadata "go-wind-ops/pkg/metadata"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const projectLogWsPath = "/admin/v1/ws/projects/log"

// ProjectLogWsHandler 项目日志实时 tail WS 处理器。
type ProjectLogWsHandler struct {
	client opsV1.ProjectServiceClient
	redis  *redis.Client

	upgrader websocket.Upgrader
}

// NewProjectLogWsHandler 构造处理器。
func NewProjectLogWsHandler(
	ctx *bootstrap.Context,
	redisClient *redis.Client,
	client opsV1.ProjectServiceClient,
) *ProjectLogWsHandler {
	_ = ctx
	return &ProjectLogWsHandler{
		client: client,
		redis:  redisClient,
		upgrader: websocket.Upgrader{
			// Origin 同源校验在 ServeHTTP 显式做。
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

	if !sameOriginWs(r) {
		http.Error(w, "cross-origin websocket forbidden", http.StatusForbidden)
		return
	}

	// ── 一次性票据消费(签发期已完成 token 鉴权 + ops:project_manage +
	// projectId 绑定;长效 token 不进握手 URL)──
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" || h.redis == nil {
		http.Error(w, "ticket required", http.StatusBadRequest)
		return
	}
	raw, rerr := h.redis.GetDel(r.Context(), terminalTicketKey(ticket)).Result()
	if rerr != nil {
		http.Error(w, "invalid or expired ticket", http.StatusUnauthorized)
		return
	}
	var tk terminalTicket
	if err := json.Unmarshal([]byte(raw), &tk); err != nil || tk.Operator == "" || tk.Kind != TicketKindProjectLog {
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}

	projectID, perr := strconv.ParseUint(r.URL.Query().Get("projectId"), 10, 32)
	if perr != nil || projectID == 0 {
		http.Error(w, "projectId required", http.StatusBadRequest)
		return
	}
	if tk.ProjectId != uint32(projectID) {
		http.Error(w, "ticket/project mismatch", http.StatusForbidden)
		return
	}
	operator, oerr := appMetadata.DecodeOperatorMetadata(tk.Operator)
	if oerr != nil || operator == nil {
		http.Error(w, "invalid ticket operator", http.StatusUnauthorized)
		return
	}

	intervalMs := 2000
	if v,VERR := strconv.Atoi(r.URL.Query().Get("interval")); VERR == nil && v >= 1000 && v <= 10000 {
		intervalMs = v
	}

	// 操作者身份以出站 gRPC 元数据头携带(轮询调用不经 metadata.Client
	// 中间件传播路径):core 侧据此构建租户 viewer,防止跨租户读项目日志。
	callCtx := r.Context()
	{
		if encoded, encErr := appMetadata.EncodeOperatorMetadata(operator); encErr == nil {
			callCtx = grpcmd.AppendToOutgoingContext(r.Context(), "x-md-global-operator", encoded)
		}
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

		resp, err := h.client.GetProjectLog(callCtx, &opsV1.GetProjectLogRequest{
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
