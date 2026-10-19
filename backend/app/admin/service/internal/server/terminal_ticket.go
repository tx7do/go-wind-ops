// terminal_ticket.go Web 终端一次性握手票据签发:
// 避免长效 accessToken 进 WS 握手 URL(会落入代理/访问日志,泄露面大)。
// 流程:前端先 POST 本端点(Bearer token 鉴权 + 权限码校验)换取 30 秒一次性
// ticket(绑定 agentId),WS 以 ?ticket=&agentId= 握手,服务端 GETDEL 原子消费。
// 本路由与 WS 同样经 mux 直挂、不走鉴权中间件,鉴权在此显式完成。
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	permissionV1 "go-wind-ops/api/gen/go/permission/service/v1"

	"go-wind-ops/app/admin/service/internal/authzcheck"
	"go-wind-ops/pkg/constants"
	appMetadata "go-wind-ops/pkg/metadata"
	"go-wind-ops/pkg/middleware/auth"
	"go-wind-ops/pkg/serviceid"
)

const (
	terminalTicketPath          = "/admin/v1/ws/terminal/tickets"
	terminalTicketTTL           = 30 * time.Second
	terminalTicketAgentMismatch = "ticket/agent mismatch"

	// 票据种类:同一签发端点服务两条 WS 流,kind 决定权限码与绑定字段。
	TicketKindTerminal   = "terminal"
	TicketKindProjectLog = "project-log"
)

// terminalTicketKey Redis 键:一次性消费票据。
func terminalTicketKey(ticket string) string {
	return serviceid.ProjectName + ":wsterm:" + ticket
}

// terminalTicket 票据载荷:操作者元数据(编码) + 按种类绑定的目标。
type terminalTicket struct {
	Kind      string `json:"kind"`
	Operator  string `json:"op"`
	AgentId   string `json:"agentId,omitempty"`
	ProjectId uint32 `json:"projectId,omitempty"`
}

// TerminalTicketHandler 终端握手票据签发。
type TerminalTicketHandler struct {
	checker    auth.AccessTokenChecker
	roleClient permissionV1.RoleServiceClient
	permClient permissionV1.PermissionServiceClient
	redis      *redis.Client
}

func NewTerminalTicketHandler(
	ctx *bootstrap.Context,
	checker auth.AccessTokenChecker,
	roleClient permissionV1.RoleServiceClient,
	permClient permissionV1.PermissionServiceClient,
	redisClient *redis.Client,
) *TerminalTicketHandler {
	_ = ctx
	return &TerminalTicketHandler{
		checker:    checker,
		roleClient: roleClient,
		permClient: permClient,
		redis:      redisClient,
	}
}

// Path 注册路径(rest_server.go Handle 用)。
func (h *TerminalTicketHandler) Path() string { return terminalTicketPath }

func (h *TerminalTicketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !sameOriginWs(r) {
		http.Error(w, "cross-origin request forbidden", http.StatusForbidden)
		return
	}

	// ── Bearer token 鉴权 ──
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" || h.checker == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	valid, payload := h.checker.IsValidAccessToken(r.Context(), token, false)
	if !valid || payload == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	operator := appMetadata.NewUserOperator(
		uint64(payload.GetUserId()),
		uint64(payload.GetTenantId()),
		uint64(payload.GetOrgUnitId()),
		payload.GetDataScope(),
	)
	encoded, err := appMetadata.EncodeOperatorMetadata(operator)
	if err != nil {
		http.Error(w, "operator metadata encode failed", http.StatusInternalServerError)
		return
	}

	// ── 按种类校验并绑定目标:terminal 绑 agentId(node_manage), ──
	// ── project-log 绑 projectId(project_manage)。                ──
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = TicketKindTerminal
	}
	tk := terminalTicket{Kind: kind, Operator: encoded}
	var requiredCode string
	switch kind {
	case TicketKindTerminal:
		tk.AgentId = r.URL.Query().Get("agentId")
		if tk.AgentId == "" {
			http.Error(w, "agentId required", http.StatusBadRequest)
			return
		}
		requiredCode = constants.OpsNodeManagePermissionCode
	case TicketKindProjectLog:
		pid, perr := strconv.ParseUint(r.URL.Query().Get("projectId"), 10, 32)
		if perr != nil || pid == 0 {
			http.Error(w, "projectId required", http.StatusBadRequest)
			return
		}
		tk.ProjectId = uint32(pid)
		requiredCode = constants.OpsProjectManagePermissionCode
	default:
		http.Error(w, "unknown ticket kind", http.StatusBadRequest)
		return
	}

	// ── 权限码:与目标 WS 握手同码(票据等价于能力凭证) ──
	allowed, err := authzcheck.HasPermissionCode(r.Context(), h.roleClient, h.permClient,
		payload.GetUserId(), requiredCode)
	if err != nil {
		http.Error(w, "authorization check failed", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "forbidden: missing permission "+requiredCode, http.StatusForbidden)
		return
	}

	// ── 签发:32 字节随机 + Redis 30s TTL ──
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		http.Error(w, "ticket generation failed", http.StatusInternalServerError)
		return
	}
	ticket := hex.EncodeToString(buf)

	blob, err := json.Marshal(tk)
	if err != nil {
		http.Error(w, "ticket marshal failed", http.StatusInternalServerError)
		return
	}
	if err = h.redis.Set(r.Context(), terminalTicketKey(ticket), blob, terminalTicketTTL).Err(); err != nil {
		http.Error(w, "ticket store failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ticket": ticket, "expiresIn": int(terminalTicketTTL.Seconds())})
}
