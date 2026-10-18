package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/go-kratos/kratos/v2/log"
	kratosHTTP "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
)

// MqttAuthServer EMQX Broker 级认证的最小 HTTP 服务。
// EMQX 配置 HTTP 认证后,agent CONNECT 时回调本端点:
//   POST /api/v1/mqtt/auth  {username, password, clientid}
//   → 校验 SHA-256(password) 与 ops_agent_nodes.token_hash 是否匹配
//   → 首次出现按 TOFU 注册(与 MQTT 信令层准入共享同一张表和同一个 token)
//
// ACL 由 EMQX authorization 规则实现(agent 只能订阅/发布自己的主题前缀),
// 不经过本端点。
type MqttAuthServer struct {
	repo *data.AgentNodeRepo
	log  *log.Helper
}

func NewMqttAuthServer(ctx *bootstrap.Context, repo *data.AgentNodeRepo) *kratosHTTP.Server {
	cfg := ctx.GetConfig()
	if cfg == nil || cfg.Server == nil || cfg.Server.Rest == nil {
		return nil
	}

	h := &MqttAuthServer{repo: repo, log: ctx.NewLoggerHelper("mqtt-auth/core-service")}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mqtt/auth", h.handleAuth)

	srv := kratosHTTP.NewServer(kratosHTTP.Address(cfg.Server.Rest.GetAddr()))
	srv.HandlePrefix("/", mux)

	return srv
}

func (h *MqttAuthServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	var req mqttAuthBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		h.writeDeny(w)
		return
	}

	ctx := r.Context()
	node, err := h.repo.GetByAgentID(ctx, req.Username)
	if err != nil {
		h.log.Errorf("auth lookup [%s]: %s", req.Username, err.Error())
		h.writeDeny(w)
		return
	}

	tokenHash := sha256Hex(req.Password)
	if node == nil {
		// TOFU 首连注册
		if _, err = h.repo.Enrol(ctx, req.Username, tokenHash, nil); err != nil {
			h.writeDeny(w)
			return
		}
		h.log.Infof("mqtt broker auth: agent [%s] enrolled (TOFU)", req.Username)
		h.writeAllow(w)
		return
	}

	if node.TokenHash == tokenHash {
		h.writeAllow(w)
		return
	}

	h.log.Warnf("mqtt broker auth: agent [%s] DENIED", req.Username)
	h.writeDeny(w)
}

func (h *MqttAuthServer) writeAllow(w http.ResponseWriter) {
	h.writeJSON(w, map[string]string{"result": "allow"})
}

func (h *MqttAuthServer) writeDeny(w http.ResponseWriter) {
	h.writeJSON(w, map[string]string{"result": "deny"})
}

func (h *MqttAuthServer) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type mqttAuthBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ClientID string `json:"clientid"`
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

