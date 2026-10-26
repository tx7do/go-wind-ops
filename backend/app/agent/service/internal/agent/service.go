// Package agent 是运行在受管机器上的 agent 应用服务:经 MQTT 与 core 保持
// 信令长连接(准入认证 → 周期心跳 → 指令接收/执行/结果回传),大文件经
// 预签名 URL 与对象存储直传直拉。本进程不注册服务发现、不连数据库。
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	kratosLog "github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-transport/broker"

	"go-wind-ops/pkg/machine"
	"go-wind-ops/pkg/signal"
	"go-wind-ops/pkg/topic"
)

const (
	// heartbeatInterval 心跳周期;core 侧超过 offlineAfter(默认 3 个周期)
	// 未收到心跳即标记节点离线。
	heartbeatInterval = 30 * time.Second
	// authRetryInterval/authRetryMax 准入请求重试:未认证前周期性重发,
	// 规避"agent 先于 core 连上 broker,首条 auth/request 被丢弃"的启动竞争。
	authRetryInterval = 5 * time.Second
	authRetryMax      = 12
	// defaultTokenFile token 默认落盘位置(进程工作目录;注意不能放进 -c 指向的
	// configs 目录——kratos-bootstrap 配置加载器会扫描该目录下所有文件)。
	defaultTokenFile = "auth.token"
)

// CommandHandler 指令处理器:按 action 注册,返回值作为 CommandResult.Data。
type CommandHandler func(ctx context.Context, env *signal.CommandEnvelope) (any, error)

// seenRequestIDCap request_id 去重环形容量:同 ID 重复投递(EMQX QoS1 重发/
// 重放攻击)只执行一次;容量按"指令峰值速率 × 台账超时时长"富余估算。
const seenRequestIDCap = 4096

// Service agent 应用服务。
type Service struct {
	ctx *bootstrap.Context

	agentID   string
	version   string
	tokenPath string

	mu       sync.RWMutex
	token    string
	handlers map[signal.CommandAction]CommandHandler

	mqttBroker atomic.Value // broker.Broker
	authed     atomic.Bool
	// authedEver 进程生命周期内是否通过过准入:首认证前不接受指令(封死
	// "未认证也能执行指令"的窗口);重连重认证不回退该标记,断线重连后的
	// 排队指令不受影响。
	authedEver atomic.Bool

	// signKey 指令信封 HMAC 签名密钥(COMMAND_SIGN_KEY,空=未签名模式)。
	signKey string

	// request_id 去重环形缓冲(配合 seenRing 轮转淘汰)。
	seenMu   sync.Mutex
	seen     map[string]struct{}
	seenRing []string
	seenIdx  int

	terminal *terminalSessionManager
	plugins  []pluginDef

	stopCh chan struct{}
}

// NewService 构造 agent 服务:确定机器 ID、加载或生成本地 token、
// 注册内置指令处理器。
func NewService(ctx *bootstrap.Context) *Service {
	version := ""
	if ai := ctx.GetAppInfo(); ai != nil {
		version = ai.GetVersion()
	}

	tokenPath := os.Getenv("AGENT_AUTH_TOKEN_FILE")
	if tokenPath == "" {
		tokenPath = defaultTokenFile
	}

	s := &Service{
		ctx:       ctx,
		agentID:   machine.MachineID(),
		version:   version,
		tokenPath: tokenPath,
		handlers:  make(map[signal.CommandAction]CommandHandler),
		signKey:   signal.SignKeyFromEnv(),
		seen:      make(map[string]struct{}, seenRequestIDCap),
		seenRing:  make([]string, seenRequestIDCap),
		stopCh:    make(chan struct{}),
	}
	if s.signKey == "" {
		ctx.NewLoggerHelper("agent/service").Warnf("%s 未配置,指令不做 HMAC 校验——接受未签名指令,仅限开发环境,生产必须与 core 一致配置", signal.SignKeyEnv)
	}

	s.RegisterHandler(signal.CommandActionPing, s.handlePing)
	s.registerProjectHandlers()
	s.registerFileHandlers()
	s.RegisterHandler(signal.CommandActionScriptExec, s.handleScriptExec)
	s.RegisterHandler(signal.CommandActionDistApply, s.handleDistApply)
	s.registerDockerHandlers()
	s.registerPluginHandlers()
	s.registerTerminalHandlers()
	s.registerBackupHandlers()
	s.registerProcessHandlers()
	s.registerAgentUpgradeHandlers()

	ctx.NewLoggerHelper("agent/service").Infof("agent initialized (agentId=%s version=%s tokenFile=%s)", s.agentID, s.version, tokenPath)
	return s
}

func (s *Service) AgentID() string { return s.agentID }

// RegisterHandler 注册指令处理器(业务轮按域扩展入口)。
func (s *Service) RegisterHandler(action signal.CommandAction, h CommandHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[action] = h
}

// LoadOrGenerateAuthToken 加载本地 token;不存在则生成 32 字节随机 hex 并落盘
// (文件权限 0600)。core 侧只存 SHA-256 哈希,token 明文只存在本机。
func (s *Service) LoadOrGenerateAuthToken() error {
	if data, err := os.ReadFile(s.tokenPath); err == nil {
		token := strings.TrimSpace(string(data))
		if len(token) >= 32 {
			s.mu.Lock()
			s.token = token
			s.mu.Unlock()
			return nil
		}
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(buf)

	if err := os.MkdirAll(filepath.Dir(s.tokenPath), 0o700); err != nil {
		return fmt.Errorf("mkdir for token file: %w", err)
	}
	if err := os.WriteFile(s.tokenPath, []byte(token+"\n"), 0o600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}

	s.mu.Lock()
	s.token = token
	s.mu.Unlock()

	s.ctx.NewLoggerHelper("agent/service").Infof("generated new auth token at %s", s.tokenPath)
	return nil
}

// GetAuthToken 返回当前 token(MQTT server 组装连接参数时取用)。
func (s *Service) GetAuthToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// RegisterMqttBroker 注入共享 MQTT 连接(publish 用)。
func (s *Service) RegisterMqttBroker(b broker.Broker) {
	s.mqttBroker.Store(b)
}

// OnMqttConnected 连接(含重连)建立后的回调:发布准入请求。
// 重连后会重复认证,保证断线恢复后无需重启进程。
// 若 core 侧订阅尚未就绪(启动顺序竞争)首条 auth/request 会丢,故以有界
// 重试兜底:未认证前每 authRetryInterval 重发,认证成功即停。
func (s *Service) OnMqttConnected() {
	s.authed.Store(false)
	go func() {
		log := s.log()
		for i := 0; i < authRetryMax; i++ {
			if s.authed.Load() {
				return
			}
			if err := s.publishAuthRequest(); err != nil {
				log.Warnf("publish auth request: %v", err)
			}
			select {
			case <-s.stopCh:
				return
			case <-time.After(authRetryInterval):
			}
		}
		if !s.authed.Load() {
			log.Errorf("auth request retry exhausted (%d attempts) without acknowledgement", authRetryMax)
		}
	}()
}

// Start 启动心跳循环。由 mqtt_server 在 kratos App 启动前调用;主题订阅与
// broker 注入也由其完成,本方法只负责本地准备工作与周期任务。
func (s *Service) Start(ctx context.Context) error {
	if err := s.LoadOrGenerateAuthToken(); err != nil {
		return err
	}
	s.startHeartbeatLoop(ctx)
	return nil
}

// Stop 停止心跳循环。
func (s *Service) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// ============================================================================
// 订阅处理(由 internal/server 的 mqtt_server 注册到具体主题)
// ============================================================================

// HandleAuthResponse 处理 core 的准入应答。
func (s *Service) HandleAuthResponse(ctx context.Context, _ string, _ broker.Headers, resp *signal.AgentAuthResponse) error {
	log := s.log()
	if resp == nil {
		return nil
	}
	if resp.OK {
		s.authed.Store(true)
		s.authedEver.Store(true)
		log.Infof("authenticated by core (server_time=%s)", resp.ServerTime.Format(time.RFC3339))
		return nil
	}
	s.authed.Store(false)
	// 拒绝多为 token 与库内哈希不符(重装/复制 token 文件);保留进程便于排查,
	// 不自动重新生成 token(会污染准入记录)。
	log.Errorf("auth DENIED by core: %s", resp.Reason)
	return nil
}

// HandleCommand 处理 core 下发的指令信封:目标/准入/签名/幂等四道校验
// 通过后分发到 action 处理器并回传结果。校验失败回执错误(重复投递除外),
// 让 core 台账及时终态化。
func (s *Service) HandleCommand(ctx context.Context, topicStr string, _ broker.Headers, env *signal.CommandEnvelope) error {
	log := s.log()
	if env == nil || env.RequestId == "" {
		log.Warn("received nil/invalid command envelope")
		return nil
	}

	// 目标校验:信封必须面向本 agent(防误投/改投)。
	if env.AgentId != "" && env.AgentId != s.agentID {
		log.Warnf("command agent mismatch (request_id=%s to=%s self=%s)", env.RequestId, env.AgentId, s.agentID)
		return s.replyResult(env, false, "command rejected: agent mismatch")
	}
	// 准入校验:进程首次通过 core 认证前不接受指令。
	if !s.authedEver.Load() {
		log.Warnf("command before authentication (request_id=%s)", env.RequestId)
		return s.replyResult(env, false, "command rejected: not authenticated yet")
	}
	// 签名/时效校验:COMMAND_SIGN_KEY 配置后强制(伪造/过期/前拨超限一律拒绝)。
	if err := signal.VerifyEnvelope(env, s.signKey, time.Now()); err != nil {
		log.Warnf("command signature rejected (request_id=%s): %v", env.RequestId, err)
		return s.replyResult(env, false, "command rejected: "+err.Error())
	}
	// 幂等去重:同 request_id 只执行一次;重复投递不回执、不执行
	// (回执会与首次执行的回执竞争台账终态,丢弃由超时链路兜底)。
	if !s.markSeenRequestID(env.RequestId) {
		log.Warnf("duplicate command dropped (request_id=%s action=%s)", env.RequestId, env.Action)
		return nil
	}
	log.Infof("command received (request_id=%s action=%s)", env.RequestId, env.Action)

	result := signal.CommandResult{
		RequestId:  env.RequestId,
		AgentId:    s.agentID,
		Action:     env.Action,
		FinishedAt: time.Now(),
	}

	s.mu.RLock()
	handler, ok := s.handlers[env.Action]
	s.mu.RUnlock()

	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("unsupported action %q", env.Action)
	} else if data, err := handler(ctx, env); err != nil {
		result.Success = false
		result.Error = err.Error()
	} else if data != nil {
		if encoded, encErr := json.Marshal(data); encErr == nil {
			result.Data = encoded
		}
		result.Success = true
	} else {
		result.Success = true
	}

	return s.publish(topic.MakeMqttTopicCommandResult(s.agentID), &result)
}

// replyResult 回传一条失败的执行结果(校验拒绝路径用)。
func (s *Service) replyResult(env *signal.CommandEnvelope, success bool, errMsg string) error {
	return s.publish(topic.MakeMqttTopicCommandResult(s.agentID), &signal.CommandResult{
		RequestId:  env.RequestId,
		AgentId:    s.agentID,
		Action:     env.Action,
		Success:    success,
		Error:      errMsg,
		FinishedAt: time.Now(),
	})
}

// markSeenRequestID 登记并判定 request_id 是否首次出现(环形淘汰最旧记录)。
func (s *Service) markSeenRequestID(id string) bool {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, dup := s.seen[id]; dup {
		return false
	}
	if old := s.seenRing[s.seenIdx]; old != "" {
		delete(s.seen, old)
	}
	s.seenRing[s.seenIdx] = id
	s.seen[id] = struct{}{}
	s.seenIdx = (s.seenIdx + 1) % len(s.seenRing)
	return true
}

// ============================================================================
// 内部实现
// ============================================================================

func (s *Service) handlePing(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := signal.PingPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode ping payload: %w", err)
		}
	}
	return signal.PingResult{
		SentAt:     payload.SentAt,
		ReceivedAt: time.Now(),
		AgentTime:  time.Now(),
	}, nil
}

func (s *Service) publishAuthRequest() error {
	req := signal.AgentAuthRequest{
		AgentId: s.agentID,
		Token:   s.GetAuthToken(),
		Info:    machine.Fingerprint(s.version),
	}
	return s.publish(topic.MakeMqttTopicAuth(s.agentID, true), &req)
}

func (s *Service) publish(topicStr string, msg any) error {
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return fmt.Errorf("mqtt broker not ready")
	}
	return b.Publish(context.Background(), topicStr, broker.NewMessage(msg))
}

// startHeartbeatLoop 周期上报心跳(内含指标快照);未通过准入前不发。
func (s *Service) startHeartbeatLoop(ctx context.Context) {
	go func() {
		log := s.log()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				if !s.authed.Load() {
					continue
				}
				hb := &signal.Heartbeat{
					AgentId:   s.agentID,
					Timestamp: time.Now(),
					Version:   s.version,
					Metrics:   machine.Snapshot(),
				}
				if err := s.publish(topic.MakeMqttTopicHeartbeat(s.agentID), hb); err != nil {
					log.Warnf("publish heartbeat: %v", err)
				}
			}
		}
	}()
}

func (s *Service) log() *kratosLog.Helper { return s.ctx.NewLoggerHelper("agent/service") }
