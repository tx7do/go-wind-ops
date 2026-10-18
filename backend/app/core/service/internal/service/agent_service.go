package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/kratos-transport/broker"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"
	"go-wind-ops/pkg/signal"
	"go-wind-ops/pkg/topic"

	storageV1 "go-wind-ops/api/gen/go/storage/service/v1"
)

const (
	// offlineAfterNodes 多久没心跳判定离线(3 个心跳周期)。
	offlineAfter = 90 * time.Second
	// offlineSweepInterval 离线扫描周期。
	offlineSweepInterval = 30 * time.Second
)

// pendingCommand 已下发未回执的指令(本轮内存态;业务轮落库审计)。
type pendingCommand struct {
	Action    signal.CommandAction
	Operator  uint32
	IssuedAt  time.Time
}

// AgentService core 侧 agent 信令处理服务:准入认证、心跳落库、指令下发
// 与结果回执。经 MQTT 与 agent 通讯,只依赖 AgentNodeRepo 与 MinIO 客户端。
type AgentService struct {
	ctx       *bootstrap.Context
	repo      *data.AgentNodeRepo
	ossClient *oss.MinIOClient
	log       *log.Helper

	mqttBroker atomic.Value // broker.Broker

	mu       sync.Mutex
	pending  map[string]*pendingCommand // request_id -> pending
}

func NewAgentService(ctx *bootstrap.Context, repo *data.AgentNodeRepo, ossClient *oss.MinIOClient) *AgentService {
	return &AgentService{
		ctx:       ctx,
		repo:      repo,
		ossClient: ossClient,
		log:       ctx.NewLoggerHelper("agent/service/core-service"),
		pending:   make(map[string]*pendingCommand),
	}
}

// RegisterMqttBroker 注入共享 MQTT 连接(publish 用)。
func (s *AgentService) RegisterMqttBroker(b broker.Broker) {
	s.mqttBroker.Store(b)
}

// Start 启动离线扫描循环(mqtt_server 装配时调用;多副本并发执行幂等)。
func (s *AgentService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(offlineSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.repo.MarkStaleOffline(ctx, time.Now().Add(-offlineAfter))
				if err != nil {
					s.log.Warnf("offline sweep: %s", err.Error())
				} else if n > 0 {
					s.log.Infof("offline sweep: marked %d agent(s) offline", n)
				}
				s.prunePending()
			}
		}
	}()
}

// ============================================================================
// MQTT 订阅处理(由 internal/server/mqtt_server.go 注册,均为 $share 共享订阅)
// ============================================================================

// HandleAgentAuth 处理 agent 准入请求。
// 准入模型(本轮):SHA-256(token) 与库内哈希一致放行;agent 首次出现按
// TOFU 信任首连落库;拒绝则回 denial(预留 EMQX webhook/ACL 升级位)。
func (s *AgentService) HandleAgentAuth(ctx context.Context, topicStr string, _ broker.Headers, req *signal.AgentAuthRequest) error {
	if req == nil || req.AgentId == "" || req.Token == "" {
		s.log.Warn("invalid auth request")
		return nil
	}

	tokenHash := hashToken(req.Token)
	resp := &signal.AgentAuthResponse{ServerTime: time.Now()}

	node, err := s.repo.GetByAgentID(ctx, req.AgentId)
	if err != nil {
		resp.Reason = "internal error"
		s.replyAuth(req.AgentId, resp)
		return err
	}

	switch {
	case node == nil:
		// TOFU 首连注册
		if _, err = s.repo.Enrol(ctx, req.AgentId, tokenHash, &req.Info); err != nil {
			resp.Reason = "internal error"
			s.replyAuth(req.AgentId, resp)
			return err
		}
		resp.OK = true
		s.log.Infof("agent [%s] enrolled (TOFU) hostname=%s os=%s/%s version=%s",
			req.AgentId, req.Info.Hostname, req.Info.OS, req.Info.Arch, req.Info.AgentVersion)

	case node.TokenHash == tokenHash:
		if err = s.repo.UpdateAuthSuccess(ctx, node.ID, &req.Info); err != nil {
			resp.Reason = "internal error"
			s.replyAuth(req.AgentId, resp)
			return err
		}
		resp.OK = true
		s.log.Infof("agent [%s] authenticated", req.AgentId)

	default:
		resp.Reason = "token mismatch"
		s.log.Warnf("agent [%s] auth DENIED: token mismatch", req.AgentId)
	}

	return s.replyAuth(req.AgentId, resp)
}

// HandleHeartbeat 处理心跳:刷新在线状态/最后心跳时间/指标快照。
// 未准入节点的心跳忽略(保持其离线态)。
func (s *AgentService) HandleHeartbeat(ctx context.Context, topicStr string, _ broker.Headers, hb *signal.Heartbeat) error {
	if hb == nil || hb.AgentId == "" {
		return nil
	}
	if err := s.repo.TouchHeartbeat(ctx, hb.AgentId, &hb.Metrics); err != nil {
		return err
	}
	s.log.Debugf("heartbeat from [%s] cpu=%.1f%% mem=%.1f%% disk=%.1f%%",
		hb.AgentId, hb.Metrics.CPUPercent, hb.Metrics.MemoryUsedPercent, hb.Metrics.DiskUsedPercent)
	return nil
}

// HandleCommandResult 处理指令执行回执,与 pending 对账(本轮记日志)。
func (s *AgentService) HandleCommandResult(ctx context.Context, topicStr string, _ broker.Headers, res *signal.CommandResult) error {
	if res == nil || res.RequestId == "" {
		return nil
	}

	s.mu.Lock()
	pending, ok := s.pending[res.RequestId]
	if ok {
		delete(s.pending, res.RequestId)
	}
	s.mu.Unlock()

	if !ok {
		s.log.Warnf("command result without pending record (request_id=%s agent=%s action=%s)",
			res.RequestId, res.AgentId, res.Action)
		return nil
	}

	if res.Success {
		s.log.Infof("command result SUCCESS (request_id=%s agent=%s action=%s data=%s)",
			res.RequestId, res.AgentId, res.Action, string(res.Data))
	} else {
		s.log.Errorf("command result FAILED (request_id=%s agent=%s action=%s error=%s)",
			res.RequestId, res.AgentId, res.Action, res.Error)
	}
	_ = pending
	return nil
}

// ============================================================================
// 指令下发(供 admin/front BFF 经 gRPC 调用;业务轮 proto 化后暴露 REST)
// ============================================================================

// PublishCommand 向指定 agent 下发指令,返回幂等 request_id。
func (s *AgentService) PublishCommand(ctx context.Context, agentId string, action signal.CommandAction, payload any, operatorId uint32) (string, error) {
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return "", fmt.Errorf("mqtt broker not ready")
	}

	requestId := id.NewGUIDv4(false)
	env := &signal.CommandEnvelope{
		RequestId:  requestId,
		AgentId:    agentId,
		Action:     action,
		IssuedAt:   time.Now(),
		OperatorId: operatorId,
	}
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal command payload: %w", err)
		}
		env.Payload = encoded
	}

	s.mu.Lock()
	s.pending[requestId] = &pendingCommand{
		Action:   action,
		Operator: operatorId,
		IssuedAt: env.IssuedAt,
	}
	s.mu.Unlock()

	if err := b.Publish(ctx, topic.MakeMqttTopicCommand(agentId), broker.NewMessage(env)); err != nil {
		s.mu.Lock()
		delete(s.pending, requestId)
		s.mu.Unlock()
		return "", err
	}

	s.log.Infof("command dispatched (request_id=%s agent=%s action=%s)", requestId, agentId, action)
	return requestId, nil
}

// IssueUploadPresignedUrl 为 agent 签发对象存储预签名上传 URL
// (文件传输信令的 core 侧一半;业务轮经 file/response 主题下发)。
func (s *AgentService) IssueUploadPresignedUrl(ctx context.Context, fileName, fileDirectory, contentType string, expireSeconds int32) (url string, objectKey string, err error) {
	if s.ossClient == nil {
		return "", "", fmt.Errorf("oss client not configured")
	}
	req := &storageV1.GetUploadPresignedUrlRequest{
		FileName:      &fileName,
		FileDirectory: &fileDirectory,
		ExpireSeconds: &expireSeconds,
	}
	if contentType != "" {
		req.ContentType = &contentType
	}
	resp, err := s.ossClient.GetUploadPresignedUrl(ctx, req)
	if err != nil {
		return "", "", err
	}
	return resp.UploadUrl, resp.ObjectName, nil
}

// ============================================================================
// 内部实现
// ============================================================================

func (s *AgentService) replyAuth(agentId string, resp *signal.AgentAuthResponse) error {
	b, ok := s.mqttBroker.Load().(broker.Broker)
	if !ok || b == nil {
		return fmt.Errorf("mqtt broker not ready")
	}
	return b.Publish(context.Background(), topic.MakeMqttTopicAuth(agentId, false), broker.NewMessage(resp))
}

// prunePending 清理超时未回执的指令(10 分钟兜底,防 map 无界增长)。
func (s *AgentService) prunePending() {
	deadline := time.Now().Add(-10 * time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	for requestId, p := range s.pending {
		if p.IssuedAt.Before(deadline) {
			s.log.Warnf("command timed out without result (request_id=%s action=%s)", requestId, p.Action)
			delete(s.pending, requestId)
		}
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
