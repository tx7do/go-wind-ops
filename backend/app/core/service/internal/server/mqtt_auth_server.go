// mqtt_auth_server.go EMQX Broker 级认证与授权的最小 HTTP 服务。
// EMQX 配置 HTTP 认证/授权后回调本端点:
//   POST /api/v1/mqtt/auth {username, password, clientid}
//     → username == core 自身账号:与 core 配置的 MQTT 凭证比对
//     → 否则按 agent 处理:校验 SHA-256(password) 与 ops_agent_nodes.token_hash
//       是否匹配;首次出现按 TOFU 注册(与 MQTT 信令层准入共享同一张表和同一个
//       token——agent 以本机 token 作为 MQTT 口令连接)
//   POST /api/v1/mqtt/acl {username, topic, action}
//     → core:放行 ops/ 前缀全部主题;agent:仅放行 ops/agent/{自身用户名}/#
//     → 其余(含 $SYS)一律拒绝
//
// 回调密钥:MQTT_AUTH_WEBHOOK_SECRET 非空时,两个端点要求携带共享密钥
// (URL 查询参数 ?secret= 或请求头 X-Mqtt-Webhook-Secret,常时比较),
// 防止内网直达 :9500 直调 TOFU 注册伪造 agent;为空则跳过(仅限开发)。
// compose 里 EMQX 回调 URL 已拼入 ?secret=,两端经同一环境变量装配。
//
// 回执契约(EMQX 5.x HTTP 认证/授权):{"result":"allow"} / {"result":"deny"}。
package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	kratosHTTP "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/configenv"
)

// MqttAuthWebhookSecretEnv 回调共享密钥的环境变量名。
const MqttAuthWebhookSecretEnv = "MQTT_AUTH_WEBHOOK_SECRET"

// mqttWebhookSecretHeader 与 URL 查询参数二选一的请求头名。
const mqttWebhookSecretHeader = "X-Mqtt-Webhook-Secret"

// MqttAuthServer EMQX Broker 级认证/授权 webhook。
type MqttAuthServer struct {
	repo *data.AgentNodeRepo
	log  *log.Helper

	coreMqttUsername string
	coreMqttPassword string
	webhookSecret    string
}

func NewMqttAuthServer(ctx *bootstrap.Context, repo *data.AgentNodeRepo) *kratosHTTP.Server {
	cfg := ctx.GetConfig()
	if cfg == nil || cfg.Server == nil || cfg.Server.Rest == nil {
		return nil
	}

	h := &MqttAuthServer{repo: repo, log: ctx.NewLoggerHelper("mqtt-auth/core-service")}
	// env 覆盖必须在消费点做(GetConfig() 返回深拷贝);与 broker 连接消费点
	// 各自调用同一 env,保证回调比对值与连接凭证一致。
	configenv.ApplyMqtt(cfg)
	if cfg.Server.Mqtt != nil {
		h.coreMqttUsername = cfg.Server.Mqtt.GetUsername()
		h.coreMqttPassword = cfg.Server.Mqtt.GetPassword()
	}
	h.webhookSecret = strings.TrimSpace(os.Getenv(MqttAuthWebhookSecretEnv))
	if h.webhookSecret == "" {
		h.log.Warnf("%s 未配置,EMQX 认证/授权回调未启用共享密钥——仅限开发环境,生产必须配置", MqttAuthWebhookSecretEnv)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mqtt/auth", h.handleAuth)
	mux.HandleFunc("POST /api/v1/mqtt/acl", h.handleAcl)

	srv := kratosHTTP.NewServer(kratosHTTP.Address(cfg.Server.Rest.GetAddr()))
	srv.HandlePrefix("/", mux)

	return srv
}

// checkWebhookSecret 校验回调共享密钥:未配置即放行(开发模式);已配置时
// 接受 URL 查询参数 ?secret= 或请求头,常时比较,不符拒绝。
func (h *MqttAuthServer) checkWebhookSecret(r *http.Request) bool {
	if h.webhookSecret == "" {
		return true
	}
	got := r.URL.Query().Get("secret")
	if got == "" {
		got = r.Header.Get(mqttWebhookSecretHeader)
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.webhookSecret)) == 1
}

func (h *MqttAuthServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !h.checkWebhookSecret(r) {
		h.log.Warnf("mqtt broker auth: webhook secret mismatch (remote=%s)", r.RemoteAddr)
		h.writeDeny(w)
		return
	}

	var req mqttAuthBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		h.writeDeny(w)
		return
	}

	// core 自身账号:与配置比对,不入 agent 表(常时比较,避免计时侧信道)
	if h.coreMqttUsername != "" && req.Username == h.coreMqttUsername {
		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.coreMqttPassword)) == 1 {
			h.writeAllow(w)
			return
		}
		h.log.Warnf("mqtt broker auth: core account DENIED (clientid=%s)", req.ClientID)
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

	if subtle.ConstantTimeCompare([]byte(node.TokenHash), []byte(tokenHash)) == 1 {
		h.writeAllow(w)
		return
	}

	h.log.Warnf("mqtt broker auth: agent [%s] DENIED", req.Username)
	h.writeDeny(w)
}

func (h *MqttAuthServer) handleAcl(w http.ResponseWriter, r *http.Request) {
	if !h.checkWebhookSecret(r) {
		h.log.Warnf("mqtt broker acl: webhook secret mismatch (remote=%s)", r.RemoteAddr)
		h.writeDeny(w)
		return
	}

	var req mqttAclBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Topic == "" {
		h.writeDeny(w)
		return
	}
	if authorizeMqttTopic(h.coreMqttUsername, req.Username, req.Topic) {
		h.writeAllow(w)
		return
	}
	h.log.Warnf("mqtt broker acl: [%s] DENIED %s %s", req.Username, req.Action, req.Topic)
	h.writeDeny(w)
}

// authorizeMqttTopic 主题级授权判定(纯函数,可单测):
//   - core 账号:ops/ 前缀全放行(含 $share 包装的订阅过滤器);
//   - agent(用户名=agentId):仅 ops/agent/{自身用户名}/...;
//   - 其余($SYS、跨 agent 主题、空用户)拒绝。
func authorizeMqttTopic(coreUser, username, topic string) bool {
	if username == "" || topic == "" {
		return false
	}

	// 共享订阅过滤器 $share/<group>/<filter>:授权按剥壳后的真实过滤器判定
	t := topic
	if strings.HasPrefix(t, "$share/") {
		parts := strings.SplitN(t, "/", 3)
		if len(parts) < 3 || parts[2] == "" {
			return false
		}
		t = parts[2]
	}

	if coreUser != "" && username == coreUser {
		return strings.HasPrefix(t, "ops/")
	}

	segs := strings.Split(t, "/")
	if len(segs) < 3 || segs[0] != "ops" || segs[1] != "agent" {
		return false
	}
	return segs[2] == username
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

type mqttAclBody struct {
	Username string `json:"username"`
	Topic    string `json:"topic"`
	Action   string `json:"action"`
	ClientID string `json:"clientid"`
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
