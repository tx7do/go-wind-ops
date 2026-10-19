// terminal_ws.go Web 终端 WebSocket 端点:
// 浏览器 WS ↔ 本服务(一次性票据握手) ↔ core TerminalService(gRPC bidi) ↔ MQTT ↔ agent。
//
// 浏览器 WebSocket API 无法携带 Authorization 头,而长效 token 放查询参数会
// 落入代理/访问日志——握手改用 30 秒一次性票据:前端先 POST
// /admin/v1/ws/terminal/tickets(Bearer token 鉴权 + ops:node_manage 权限码
// 校验,绑定 agentId),WS 以 ?ticket=&agentId= 握手,本端 GETDEL 原子消费,
// 长效 token 不再出现握手 URL。操作者身份经 x-md-global-operator 元数据
// 注入 gRPC 流,core 侧台账/会话审计据此归因。
//
// 线路协议(与前端约定):
//   - 服务端 → 浏览器:二进制帧 = 终端输出原始字节;文本帧 = 控制事件 JSON
//     ({"event":"OPENED"} / {"event":"CLOSED","error":"..."});
//   - 浏览器 → 服务端:二进制帧 = 键盘输入原始字节;文本帧 = 控制事件 JSON
//     ({"event":"RESIZE","cols":N,"rows":M})。
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	grpcmd "google.golang.org/grpc/metadata"

	appMetadata "go-wind-ops/pkg/metadata"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const terminalWsPath = "/admin/v1/ws/terminal"

// TerminalWsHandler 终端 WS 处理器。
type TerminalWsHandler struct {
	client opsV1.TerminalServiceClient
	redis  *redis.Client

	upgrader websocket.Upgrader
}

// NewTerminalWsHandler 构造终端 WS 处理器。
func NewTerminalWsHandler(
	ctx *bootstrap.Context,
	redisClient *redis.Client,
	terminalClient opsV1.TerminalServiceClient,
) *TerminalWsHandler {
	_ = ctx
	return &TerminalWsHandler{
		client: terminalClient,
		redis:  redisClient,
		upgrader: websocket.Upgrader{
			// Origin 同源校验在 ServeHTTP 显式做(WS 握手无凭据头,Origin
			// 检查是唯一的跨站防线);此处放开由自研逻辑接管。
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Path 注册路径(rest_server.go Handle 用)。
func (h *TerminalWsHandler) Path() string { return terminalWsPath }

// strPtr 指针简写。
func strPtrWs(v string) *string { return &v }

func (h *TerminalWsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ── 同源校验:浏览器 WS 握手不带自定义头,Origin 是唯一跨站信号;
	// 非浏览器客户端(无 Origin)不受此限,凭 token 鉴权。──
	if !sameOriginWs(r) {
		http.Error(w, "cross-origin websocket forbidden", http.StatusForbidden)
		return
	}

	// ── 一次性票据消费(GETDEL 原子;签发时已完成 token 鉴权 + 权限码校验 +
	// agentId 绑定,这里只验真实性与一致性,长效 token 不进握手 URL)──
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" || h.redis == nil {
		http.Error(w, "ticket required", http.StatusBadRequest)
		return
	}
	agentId := r.URL.Query().Get("agentId")
	if agentId == "" {
		http.Error(w, "agentId required", http.StatusBadRequest)
		return
	}
	raw, err := h.redis.GetDel(r.Context(), terminalTicketKey(ticket)).Result()
	if err != nil {
		http.Error(w, "invalid or expired ticket", http.StatusUnauthorized)
		return
	}
	var tk terminalTicket
	if err = json.Unmarshal([]byte(raw), &tk); err != nil || tk.Operator == "" {
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}
	if tk.Kind != TicketKindTerminal {
		http.Error(w, "ticket kind mismatch", http.StatusForbidden)
		return
	}
	if tk.AgentId != agentId {
		http.Error(w, terminalTicketAgentMismatch, http.StatusForbidden)
		return
	}
	operator, err := appMetadata.DecodeOperatorMetadata(tk.Operator)
	if err != nil || operator == nil {
		http.Error(w, "invalid ticket operator", http.StatusUnauthorized)
		return
	}

	// ── 升级 WebSocket ──
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade 已写错误响应
	}
	defer conn.Close()

	// ── 开 gRPC 双向流:操作者身份以 x-md-global-operator 出站头直塞
	// (kratos 流式 RPC 不执行 metadata.Client 中间件,必须走 grpc 原生
	// 出站元数据);core 侧据此构建租户 viewer 并做台账/会话归因。──
	encoded, encErr := appMetadata.EncodeOperatorMetadata(operator)
	if encErr != nil {
		http.Error(w, "operator metadata encode failed", http.StatusInternalServerError)
		return
	}
	streamCtx, cancel := context.WithCancel(
		grpcmd.AppendToOutgoingContext(context.Background(), "x-md-global-operator", encoded))
	defer cancel()
	stream, err := h.client.Channel(streamCtx)
	if err != nil {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "terminal stream open failed"),
			time.Now().Add(time.Second))
		return
	}

	sessionId := id.NewGUIDv4(false)
	first := &opsV1.TerminalChunk{
		AgentId:   &agentId,
		SessionId: &sessionId,
		Event:     strPtrWs("OPEN"),
	}
	if err = stream.Send(first); err != nil {
		return
	}

	// gRPC → WS
	go func() {
		for {
			chunk, rerr := stream.Recv()
			if rerr != nil {
				cancel() // 流断开,同步终止读泵
				return
			}
			if len(chunk.GetData()) > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, chunk.GetData()); werr != nil {
					cancel()
					return
				}
			}
			switch chunk.GetEvent() {
			case "CLOSED":
				_ = conn.WriteMessage(websocket.TextMessage,
					[]byte(`{"event":"CLOSED","error":`+wsQuote(chunk.GetError())+`}`))
				cancel()
				return
			case "OPENED":
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"OPENED"}`))
			}
		}
	}()

	// WS → gRPC
	for {
		mt, data, rerr := conn.ReadMessage()
		if rerr != nil {
			cancel() // 触发 gRPC 流关闭 → core terminal_close
			return
		}
		if len(data) == 0 || mt == websocket.CloseMessage {
			continue
		}
		// 文本帧=控制事件(目前仅 RESIZE);二进制帧=键盘输入原始字节。
		if mt == websocket.TextMessage {
			if ctl, ok := wsParseControl(data); ok {
				if serr := stream.Send(ctl); serr != nil {
					cancel()
					return
				}
				continue
			}
		}
		if serr := stream.Send(&opsV1.TerminalChunk{Data: data}); serr != nil {
			cancel()
			return
		}
	}
}

// sameOriginWs 同源校验:有 Origin 头时其 host 必须与请求 Host 一致
// (大小写不敏感);无 Origin(非浏览器/同源 GET)放行。
func sameOriginWs(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// wsParseControl 解析浏览器文本控制帧(目前仅 RESIZE);非控制帧返回 false,
// 按原始字节转发。
func wsParseControl(data []byte) (*opsV1.TerminalChunk, bool) {
	var ctl struct {
		Event string `json:"event"`
		Cols  uint32 `json:"cols"`
		Rows  uint32 `json:"rows"`
	}
	if json.Unmarshal(data, &ctl) != nil || ctl.Event != "RESIZE" || ctl.Cols == 0 || ctl.Rows == 0 {
		return nil, false
	}
	return &opsV1.TerminalChunk{Event: strPtrWs("RESIZE"), Cols: &ctl.Cols, Rows: &ctl.Rows}, true
}

// wsQuote 极简 JSON 字符串转义(错误文案仅含少量可控字符)。
func wsQuote(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				out = append(out, '\\', 'u', '0', '0', hexDigit(c>>4), hexDigit(c&0xF))
			} else {
				out = append(out, c)
			}
		}
	}
	return string(append(out, '"'))
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'a' + b - 10
}
