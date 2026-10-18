package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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
	// commandTimeout 指令回执超时:超时未回执的 PENDING 台账由扫描定案 FAILED。
	commandTimeout = 10 * time.Minute
)

// AgentService core 侧 agent 信令处理服务:准入认证、心跳落库、指令下发
// 与结果回执。指令台账落库(ops_command_records),按 request_id 幂等。
type AgentService struct {
	ctx            *bootstrap.Context
	repo           *data.AgentNodeRepo
	recordRepo     *data.CommandRecordRepo
	projectRepo    *data.ProjectRepo
	distRecordRepo *data.DistributionRecordRepo
	metricRepo     *data.NodeMetricRepo
	ossClient      *oss.MinIOClient
	log            *log.Helper

	mqttBroker atomic.Value // broker.Broker

	// waiters 同步指令的等待队列:request_id → 回执投递通道。
	waiters sync.Map
}

func NewAgentService(ctx *bootstrap.Context, repo *data.AgentNodeRepo, recordRepo *data.CommandRecordRepo, projectRepo *data.ProjectRepo, distRecordRepo *data.DistributionRecordRepo, metricRepo *data.NodeMetricRepo, ossClient *oss.MinIOClient) *AgentService {
	return &AgentService{
		ctx:            ctx,
		repo:           repo,
		recordRepo:     recordRepo,
		projectRepo:    projectRepo,
		distRecordRepo: distRecordRepo,
		metricRepo:     metricRepo,
		ossClient:      ossClient,
		log:            ctx.NewLoggerHelper("agent/service/core-service"),
	}
}

// RegisterMqttBroker 注入共享 MQTT 连接(publish 用)。
func (s *AgentService) RegisterMqttBroker(b broker.Broker) {
	s.mqttBroker.Store(b)
}

// Start 启动后台扫描循环:节点离线判定 + 指令回执超时定案。
// (mqtt_server 装配时调用;多副本并发执行幂等。)
func (s *AgentService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(offlineSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := s.repo.MarkStaleOffline(ctx, time.Now().Add(-offlineAfter)); err != nil {
					s.log.Warnf("offline sweep: %s", err.Error())
				} else if n > 0 {
					s.log.Infof("offline sweep: marked %d agent(s) offline", n)
				}
				if n, err := s.recordRepo.FinalizeStalePending(ctx, time.Now().Add(-commandTimeout)); err != nil {
					s.log.Warnf("command timeout sweep: %s", err.Error())
				} else if n > 0 {
					s.log.Infof("command timeout sweep: finalized %d stale command(s)", n)
				}
				if n, err := s.distRecordRepo.FinalizeStalePending(ctx, time.Now().Add(-commandTimeout)); err != nil {
					s.log.Warnf("distribution timeout sweep: %s", err.Error())
				} else if n > 0 {
					s.log.Infof("distribution timeout sweep: finalized %d stale record(s)", n)
				}
				// 节点指标保留 7 天
				if s.metricRepo != nil {
					if n, err := s.metricRepo.CleanupOlderThan(ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
						s.log.Warnf("node metric cleanup: %s", err.Error())
					} else if n > 0 {
						s.log.Infof("node metric cleanup: removed %d old record(s)", n)
					}
				}
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

	// 指标历史入库(异步,不阻塞心跳处理)
	if s.metricRepo != nil && nodeIDFromHeartbeat(s, hb.AgentId) > 0 {
		go func() {
			bgCtx := context.Background()
			m := hb.Metrics
			_ = s.metricRepo.Create(bgCtx, nodeIDFromHeartbeat(s, hb.AgentId),
				m.CPUPercent, m.MemoryUsedPercent, m.DiskUsedPercent,
				m.MemTotalBytes, m.UptimeSeconds)
		}()
	}

	s.log.Debugf("heartbeat from [%s] cpu=%.1f%% mem=%.1f%% disk=%.1f%%",
		hb.AgentId, hb.Metrics.CPUPercent, hb.Metrics.MemoryUsedPercent, hb.Metrics.DiskUsedPercent)
	return nil
}

// nodeIDFromHeartbeat 从缓存查节点 ID(避免每次心跳都查 DB)。
var nodeIDCache sync.Map // agentId -> nodeID

func nodeIDFromHeartbeat(s *AgentService, agentID string) uint32 {
	if v, ok := nodeIDCache.Load(agentID); ok {
		return v.(uint32)
	}
	node, err := s.repo.GetByAgentID(context.Background(), agentID)
	if err != nil || node == nil {
		return 0
	}
	nodeIDCache.Store(agentID, node.ID)
	return node.ID
}

// HandleCommandResult 处理指令执行回执:按 request_id 回写指令台账终态;
// 项目类指令同时把 agent 判定的运行状态回填项目表。
func (s *AgentService) HandleCommandResult(ctx context.Context, topicStr string, _ broker.Headers, res *signal.CommandResult) error {
	if res == nil || res.RequestId == "" {
		return nil
	}

	if err := s.recordRepo.FinalizeByRequestID(ctx, res.RequestId,
		toRecordStatus(res.Success), strings.TrimSpace(res.Error), string(res.Data)); err != nil {
		s.log.Warnf("finalize command record [%s]: %s", res.RequestId, err.Error())
	}

	if res.Success && strings.HasPrefix(string(res.Action), "project_") {
		s.applyProjectOperationResult(ctx, res)
	}
	// dist_apply/docker_deploy:无论成败都要终态化分发记录(失败时带原因)
	if res.Action == signal.CommandActionDistApply || res.Action == signal.CommandActionDockerDeploy {
		s.applyDistributionResult(ctx, res)
	}

	// 同步指令等待者投递(查询类指令经此返回调用方)
	if w, ok := s.waiters.LoadAndDelete(res.RequestId); ok {
		if ch, ok := w.(chan *signal.CommandResult); ok {
			select {
			case ch <- res:
			default:
			}
		}
	}

	if res.Success {
		s.log.Infof("command result SUCCESS (request_id=%s agent=%s action=%s data=%s)",
			res.RequestId, res.AgentId, res.Action, string(res.Data))
	} else {
		s.log.Errorf("command result FAILED (request_id=%s agent=%s action=%s error=%s)",
			res.RequestId, res.AgentId, res.Action, res.Error)
	}
	return nil
}

// applyDistributionResult 把分发回执回写到分发记录(按指令幂等键对账)。
func (s *AgentService) applyDistributionResult(ctx context.Context, res *signal.CommandResult) {
	if err := s.distRecordRepo.FinalizeByCommandRequestID(ctx, res.RequestId,
		toRecordStatus(res.Success), strings.TrimSpace(res.Error), string(res.Data)); err != nil {
		s.log.Warnf("finalize distribution record [%s]: %s", res.RequestId, err.Error())
	}
}

// applyProjectOperationResult 把项目操作回执回填到项目表(last_pid/last_status)。
// 失败仅记日志,不影响台账回写。
func (s *AgentService) applyProjectOperationResult(ctx context.Context, res *signal.CommandResult) {
	var out signal.ProjectOperationResult
	if len(res.Data) == 0 || json.Unmarshal(res.Data, &out) != nil {
		return
	}
	if out.ProjectId == 0 {
		return
	}
	status := ""
	switch strings.ToUpper(out.Status) {
	case "RUNNING":
		status = "RUNNING"
	case "STOPPED":
		status = "STOPPED"
	default:
		return
	}
	if err := s.projectRepo.UpdateRuntime(ctx, out.ProjectId, out.Pid, status); err != nil {
		s.log.Warnf("apply project runtime (project=%d): %s", out.ProjectId, err.Error())
	}
}

// ============================================================================
// 指令下发(供 ProjectService/AgentNodeService 等 gRPC 服务调用)
// ============================================================================

// PublishCommand 向指定 agent 下发指令:创建 PENDING 台账行 → MQTT 发布;
// 发布失败即时定案 FAILED 并返回错误。返回幂等 request_id。
func (s *AgentService) PublishCommand(ctx context.Context, nodeID uint32, agentId string, action signal.CommandAction, payload any, operatorId uint32) (string, error) {
	return s.publish(ctx, nodeID, agentId, action, payload, operatorId, nil)
}

// PublishCommandSync 下发指令并等待回执(查询类指令用)。超时未回执返回
// 错误;迟到回执仍会由台账/分发对账链路消费。
func (s *AgentService) PublishCommandSync(ctx context.Context, nodeID uint32, agentId string, action signal.CommandAction, payload any, operatorId uint32, timeout time.Duration) (*signal.CommandResult, error) {
	waiter := make(chan *signal.CommandResult, 1)
	if _, err := s.publish(ctx, nodeID, agentId, action, payload, operatorId, waiter); err != nil {
		return nil, err
	}

	select {
	case res := <-waiter:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("command timeout after %s", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// publish 指令下发内部实现:创建 PENDING 台账行 → MQTT 发布;waiter 非空时
// 注册同步等待。
func (s *AgentService) publish(ctx context.Context, nodeID uint32, agentId string, action signal.CommandAction, payload any, operatorId uint32, waiter chan *signal.CommandResult) (string, error) {
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
	var payloadJSON string
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal command payload: %w", err)
		}
		env.Payload = encoded
		payloadJSON = string(encoded)
	}

	if _, err := s.recordRepo.Create(ctx, requestId, nodeID, agentId, string(action), payloadJSON, operatorId); err != nil {
		return "", fmt.Errorf("create command record: %w", err)
	}

	if waiter != nil {
		s.waiters.Store(requestId, waiter)
	}

	if err := b.Publish(ctx, topic.MakeMqttTopicCommand(agentId), broker.NewMessage(env)); err != nil {
		if waiter != nil {
			s.waiters.Delete(requestId)
		}
		_ = s.recordRepo.FinalizeByRequestID(ctx, requestId, "FAILED", "publish failed: "+err.Error(), "")
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

func toRecordStatus(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "FAILED"
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
