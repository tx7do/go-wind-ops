// terminal_ws.go Web 终端 WebSocket 端点:
// 浏览器 WS ↔ 本服务(参数 token 鉴权) ↔ core TerminalService(gRPC bidi) ↔ MQTT ↔ agent。
//
// 浏览器 WebSocket API 无法携带 Authorization 头,token 经查询参数传入,
// 复用 admin 的 accessTokenChecker 校验;握手后双向泵字节。
//
// 线路协议(与前端约定):
//   - 服务端 → 浏览器:二进制帧 = 终端输出原始字节;文本帧 = 控制事件 JSON
//     ({"event":"OPENED"} / {"event":"CLOSED","error":"..."});
//   - 浏览器 → 服务端:二进制或文本帧 = 键盘输入原始字节。
//
// 本路由经 kratos Server.Handle 直接注册在 mux 上,不经过鉴权中间件,
// 鉴权完全由查询参数 token 完成。
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/pkg/middleware/auth"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

const terminalWsPath = "/admin/v1/ws/terminal"

// TerminalWsHandler 终端 WS 处理器。
type TerminalWsHandler struct {
	checker auth.AccessTokenChecker
	client  opsV1.TerminalServiceClient

	upgrader websocket.Upgrader
}

// NewTerminalWsHandler 构造终端 WS 处理器。
func NewTerminalWsHandler(ctx *bootstrap.Context, checker auth.AccessTokenChecker, client opsV1.TerminalServiceClient) *TerminalWsHandler {
	_ = ctx
	return &TerminalWsHandler{
		checker: checker,
		client:  client,
		upgrader: websocket.Upgrader{
			// token 已在握手期校验,Origin 放开(WS 无同源沙箱增益)。
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

	// ── 参数 token 鉴权 ──
	token := r.URL.Query().Get("token")
	if token == "" || h.checker == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if valid, _ := h.checker.IsValidAccessToken(r.Context(), token, false); !valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	agentId := r.URL.Query().Get("agentId")
	if agentId == "" {
		http.Error(w, "agentId required", http.StatusBadRequest)
		return
	}

	// ── 升级 WebSocket ──
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade 已写错误响应
	}
	defer conn.Close()

	// ── 开 gRPC 双向流 ──
	streamCtx, cancel := context.WithCancel(context.Background())
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
		if serr := stream.Send(&opsV1.TerminalChunk{Data: data}); serr != nil {
			cancel()
			return
		}
	}
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
