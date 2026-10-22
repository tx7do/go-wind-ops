package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/minio/minio-go/v7"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/go-utils/id"
	"github.com/tx7do/kratos-transport/broker"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/agentnode"
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
	// 必须大于脚本执行上限(signal.ScriptExecTimeoutMax=600s)并留余量,否则
	// 跑满上限的脚本会先被扫成 FAILED,迟到的真实回执被幂等丢弃,流水线
	// 脚本步骤据 FAILED 误判(agent 端实际成功)。
	commandTimeout = 15 * time.Minute
	// backupCommandTimeout 备份回执超时:打包大目录+上传耗时显著高于普通指令。
	backupCommandTimeout = 30 * time.Minute
)

// AgentService core 侧 agent 信令处理服务:准入认证、心跳落库、指令下发
// 与结果回执。指令台账落库(ops_command_records),按 request_id 幂等。
type AgentService struct {
	ctx              *bootstrap.Context
	repo             *data.AgentNodeRepo
	recordRepo       *data.CommandRecordRepo
	projectRepo      *data.ProjectRepo
	distRecordRepo   *data.DistributionRecordRepo
	batchRepo        *data.ScriptBatchRepo
	backupRecordRepo *data.BackupRecordRepo
	backupTaskRepo   *data.BackupTaskRepo
	configDistRepo   *data.ConfigDistributionRepo
	metricRepo       *data.NodeMetricRepo
	alertRepo        *data.AlertRepo
	notifChannelRepo *data.NotificationChannelRepo
	deliveryRepo     *data.NotificationDeliveryRepo
	ossClient        *oss.MinIOClient
	log              *log.Helper

	mqttBroker atomic.Value // broker.Broker

	// distAdvanceHook 分发批次推进钩子(回执/超时定案后触发;wiring 晚于
	// 本服务构造,经 Setter 注入避免循环依赖)。
	distAdvanceHook atomic.Value // func(context.Context, uint32)

	// waiters 同步指令的等待队列:request_id → 回执投递通道。
	waiters sync.Map

	// terminalRoutes Web 终端输出路由:"agentId/sessionId" → 流输出通道。
	terminalRoutesMu sync.Mutex
	terminalRoutes   map[string]chan *signal.TerminalIO
}

func NewAgentService(ctx *bootstrap.Context, repo *data.AgentNodeRepo, recordRepo *data.CommandRecordRepo, projectRepo *data.ProjectRepo, distRecordRepo *data.DistributionRecordRepo, batchRepo *data.ScriptBatchRepo, backupRecordRepo *data.BackupRecordRepo, backupTaskRepo *data.BackupTaskRepo, configDistRepo *data.ConfigDistributionRepo, metricRepo *data.NodeMetricRepo, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, ossClient *oss.MinIOClient) *AgentService {
	return &AgentService{
		ctx:              ctx,
		repo:             repo,
		recordRepo:       recordRepo,
		projectRepo:      projectRepo,
		distRecordRepo:   distRecordRepo,
		batchRepo:        batchRepo,
		backupRecordRepo: backupRecordRepo,
		backupTaskRepo:   backupTaskRepo,
		configDistRepo:   configDistRepo,
		metricRepo:       metricRepo,
		alertRepo:        alertRepo,
		notifChannelRepo: notifChannelRepo,
		deliveryRepo:     deliveryRepo,
		ossClient:        ossClient,
		log:              ctx.NewLoggerHelper("agent/service/core-service"),

		terminalRoutes: make(map[string]chan *signal.TerminalIO),
	}
}

// RegisterMqttBroker 注入共享 MQTT 连接(publish 用)。
func (s *AgentService) RegisterMqttBroker(b broker.Broker) {
	s.mqttBroker.Store(b)
}

// SetDistributionAdvanceHook 注入分批发布推进钩子(distributionID 维度)。
func (s *AgentService) SetDistributionAdvanceHook(h func(context.Context, uint32)) {
	s.distAdvanceHook.Store(h)
}

func (s *AgentService) notifyDistAdvance(ctx context.Context, distributionID uint32) {
	if h, ok := s.distAdvanceHook.Load().(func(context.Context, uint32)); ok && h != nil && distributionID > 0 {
		h(ctx, distributionID)
	}
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
				cutoff := time.Now().Add(-offlineAfter)
				if stale, err := s.repo.ListStaleOnline(ctx, cutoff); err != nil {
					s.log.Warnf("offline sweep: %s", err.Error())
				} else {
					for _, node := range stale {
						flipped, err := s.repo.MarkOfflineIfStale(ctx, node.ID, cutoff)
						if err != nil {
							s.log.Warnf("offline sweep mark [%d]: %s", node.ID, err.Error())
							continue
						}
						if flipped {
							s.log.Infof("offline sweep: agent [%s] marked offline", node.AgentID)
							s.fireNodeOfflineAlert(node)
						}
					}
				}
				if n, err := s.recordRepo.FinalizeStalePending(ctx, time.Now().Add(-commandTimeout)); err != nil {
					s.log.Warnf("command timeout sweep: %s", err.Error())
				} else if n > 0 {
					s.log.Infof("command timeout sweep: finalized %d stale command(s)", n)
				}
				if distIDs, err := s.distRecordRepo.FinalizeStalePending(ctx, time.Now().Add(-commandTimeout)); err != nil {
					s.log.Warnf("distribution timeout sweep: %s", err.Error())
				} else if len(distIDs) > 0 {
					s.log.Infof("distribution timeout sweep: finalized %d stale record(s)", len(distIDs))
					for _, distID := range distIDs {
						s.notifyDistAdvance(ctx, distID)
					}
				}
				// 批次内超时未回执的指令逐条定案并对账批次计数
				if stale, err := s.recordRepo.ListStalePendingBatchItems(ctx, time.Now().Add(-commandTimeout)); err != nil {
					s.log.Warnf("batch item timeout sweep: %s", err.Error())
				} else {
					for _, rec := range stale {
						_ = s.recordRepo.FinalizeByRequestID(ctx, rec.RequestID, "FAILED", "command timeout without result", "")
						if err = s.batchRepo.ApplyItemResult(ctx, rec.BatchID, false); err != nil {
							s.log.Warnf("batch item timeout apply (batch=%d): %s", rec.BatchID, err.Error())
						}
					}
				}
				if s.backupRecordRepo != nil {
					if n, err := s.backupRecordRepo.FinalizeStalePending(ctx, time.Now().Add(-backupCommandTimeout)); err != nil {
						s.log.Warnf("backup timeout sweep: %s", err.Error())
					} else if n > 0 {
						s.log.Infof("backup timeout sweep: finalized %d stale record(s)", n)
					}
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
		wasOffline := node.Status != nil && *node.Status == agentnode.StatusOffline
		if err = s.repo.UpdateAuthSuccess(ctx, node.ID, &req.Info); err != nil {
			resp.Reason = "internal error"
			s.replyAuth(req.AgentId, resp)
			return err
		}
		resp.OK = true
		s.log.Infof("agent [%s] authenticated", req.AgentId)
		if wasOffline {
			go s.fireNodeOnlineRecovery(req.AgentId)
		}

	default:
		resp.Reason = "token mismatch"
		s.log.Warnf("agent [%s] auth DENIED: token mismatch", req.AgentId)
	}

	return s.replyAuth(req.AgentId, resp)
}

// HandleHeartbeat 处理心跳:刷新在线状态/最后心跳时间/指标快照。
// 未准入节点的心跳忽略(保持其离线态);离线节点重新上报心跳时触发恢复
// 通知(失联告警解除)。
func (s *AgentService) HandleHeartbeat(ctx context.Context, topicStr string, _ broker.Headers, hb *signal.Heartbeat) error {
	if hb == nil || hb.AgentId == "" {
		return nil
	}
	prevOffline := false
	if node, err := s.repo.GetByAgentID(ctx, hb.AgentId); err != nil {
		s.log.Warnf("heartbeat lookup [%s]: %s", hb.AgentId, err.Error())
	} else if node != nil && node.Status != nil && *node.Status == agentnode.StatusOffline {
		prevOffline = true
	}
	if err := s.repo.TouchHeartbeat(ctx, hb.AgentId, &hb.Metrics); err != nil {
		return err
	}
	if prevOffline {
		go s.fireNodeOnlineRecovery(hb.AgentId)
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
	// backup_run:终态化备份记录(大小/校验和),成功后执行保留策略清扫
	if res.Action == signal.CommandActionBackupRun {
		s.applyBackupResult(ctx, res)
	}
	// file_push:配置下发记录终态对账(文件管理器的 file_push 无对应记录,静默跳过)
	if res.Action == signal.CommandActionFilePush && s.configDistRepo != nil {
		_ = s.configDistRepo.FinalizeByCommandRequestID(ctx, res.RequestId,
			toRecordStatus(res.Success), strings.TrimSpace(res.Error))
	}
	// script_exec 且带批次标记:对账批次成败计数(幂等依赖台账仅首次定案)
	if res.Action == signal.CommandActionScriptExec && s.batchRepo != nil {
		s.applyBatchResult(ctx, res)
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

// applyDistributionResult 把分发回执回写到分发记录(按指令幂等键对账),
// 并触发分批发布推进钩子。
func (s *AgentService) applyDistributionResult(ctx context.Context, res *signal.CommandResult) {
	rec, err := s.distRecordRepo.GetByCommandRequestID(ctx, res.RequestId)
	if err != nil || rec == nil {
		s.log.Warnf("finalize distribution record [%s]: lookup failed: %v", res.RequestId, err)
		return
	}
	if err := s.distRecordRepo.FinalizeByCommandRequestID(ctx, res.RequestId,
		toRecordStatus(res.Success), strings.TrimSpace(res.Error), string(res.Data)); err != nil {
		s.log.Warnf("finalize distribution record [%s]: %s", res.RequestId, err.Error())
	}
	s.notifyDistAdvance(ctx, rec.DistributionID)
}

// applyBackupResult 把备份回执回写到备份记录(大小/校验和/终态),成功后
// 按任务保留策略清理超额历史备份。
func (s *AgentService) applyBackupResult(ctx context.Context, res *signal.CommandResult) {
	if s.backupRecordRepo == nil {
		return
	}
	var out signal.BackupRunResult
	if len(res.Data) > 0 {
		_ = json.Unmarshal(res.Data, &out)
	}
	if err := s.backupRecordRepo.FinalizeByCommandRequestID(ctx, res.RequestId,
		toRecordStatus(res.Success), strings.TrimSpace(res.Error), out.SizeBytes, out.SHA256); err != nil {
		s.log.Warnf("finalize backup record [%s]: %s", res.RequestId, err.Error())
	}
	if res.Success && out.TaskID > 0 {
		s.trimBackupRetention(ctx, out.TaskID)
	}
}

// trimBackupRetention 保留策略:删除保留份数之外的成功备份(MinIO 对象 + 记录)。
func (s *AgentService) trimBackupRetention(ctx context.Context, taskID uint32) {
	if s.backupTaskRepo == nil || s.ossClient == nil || s.ossClient.GetClient() == nil {
		return
	}
	task, err := s.backupTaskRepo.GetEntity(ctx, taskID)
	if err != nil {
		s.log.Warnf("backup retention load task [%d]: %s", taskID, err.Error())
		return
	}
	keep := task.Retention
	if keep <= 0 {
		return
	}
	excess, err := s.backupRecordRepo.ListSuccessHoldersByTask(ctx, taskID, keep)
	if err != nil {
		s.log.Warnf("backup retention list (task=%d): %s", taskID, err.Error())
		return
	}
	for _, rec := range excess {
		if err = s.ossClient.GetClient().RemoveObject(ctx, rec.Bucket, rec.ObjectKey, minio.RemoveObjectOptions{}); err != nil {
			s.log.Warnf("backup retention remove object (record=%d): %s", rec.ID, err.Error())
			continue
		}
		if err = s.backupRecordRepo.DeleteEntity(ctx, rec.ID); err != nil {
			s.log.Warnf("backup retention delete record (record=%d): %s", rec.ID, err.Error())
		}
	}
}

// applyBatchResult 批量执行回执对账:脚本指令带批次标记时累加批次计数。
func (s *AgentService) applyBatchResult(ctx context.Context, res *signal.CommandResult) {
	rec, err := s.recordRepo.GetByRequestID(ctx, res.RequestId)
	if err != nil || rec == nil || rec.BatchID == 0 {
		return
	}
	if err = s.batchRepo.ApplyItemResult(ctx, rec.BatchID, res.Success); err != nil {
		s.log.Warnf("apply batch result (request_id=%s batch=%d): %s", res.RequestId, rec.BatchID, err.Error())
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

// PresignAgentUpgradeUpload 为浏览器直传 agent 升级包签发预签名 PUT URL
// (对象键固定 agent-uploads/ 前缀,下发下载时校验防越权)。
func (s *AgentService) PresignAgentUpgradeUpload(ctx context.Context, filename string) (string, string, error) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return "", "", fmt.Errorf("oss client not configured")
	}
	objectKey := fmt.Sprintf("agent-uploads/%s%s", id.NewGUIDv4(false), strings.ToLower(filepath.Ext(filename)))
	if err := s.ossClient.EnsureBucketExists(ctx, oss.BucketFiles); err != nil {
		return "", "", fmt.Errorf("ensure bucket failed")
	}
	u, err := s.ossClient.GetClient().PresignedPutObject(ctx, oss.BucketFiles, objectKey, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("presign put failed")
	}
	return u.String(), objectKey, nil
}

// PresignAgentUpgradeDownload 校验前缀后为升级包签发预签名 GET URL。
func (s *AgentService) PresignAgentUpgradeDownload(ctx context.Context, objectKey string) (string, error) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return "", fmt.Errorf("oss client not configured")
	}
	if !strings.HasPrefix(objectKey, "agent-uploads/") || strings.Contains(objectKey, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	u, err := s.ossClient.GetClient().PresignedGetObject(ctx, oss.BucketFiles, objectKey, time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("presign get failed")
	}
	return u.String(), nil
}

// HashAgentUpgradeObject 实测计算 agent-uploads/ 下对象的 SHA-256
// (流式读取,不落盘),作为升级指令的期望摘要。
func (s *AgentService) HashAgentUpgradeObject(ctx context.Context, objectKey string) (string, error) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return "", fmt.Errorf("oss client not configured")
	}
	if !strings.HasPrefix(objectKey, "agent-uploads/") || strings.Contains(objectKey, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	obj, err := s.ossClient.GetClient().GetObject(ctx, oss.BucketFiles, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("get object: %w", err)
	}
	defer func() { _ = obj.Close() }()
	h := sha256.New()
	if _, err = io.Copy(h, obj); err != nil {
		return "", fmt.Errorf("read object: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

// fireNodeOfflineAlert 节点失联告警:source=NODE_OFFLINE,monitor_id 复用为
// 节点 ID(与预算/证书来源"存域对象 ID"的去重/恢复惯例一致),node_id 同填。
// 已有未恢复失联告警则跳过;通知走统一渠道外呼(投递台账)。
func (s *AgentService) fireNodeOfflineAlert(node *ent.AgentNode) {
	if s.alertRepo == nil {
		return
	}
	ctx := context.Background()
	if already, _ := s.alertRepo.HasUnresolvedAlert(ctx, node.ID, "NODE_OFFLINE"); already {
		return
	}
	msg := fmt.Sprintf("节点 [%s] 心跳超时已判离线(最后心跳 %s)",
		agentDisplayName(node), formatTimeOrUnknown(node.LastHeartbeatAt))
	a, err := s.alertRepo.Create(ctx, node.ID, 0, node.ID, "NODE_OFFLINE", "WARNING", msg, false)
	if err != nil {
		s.log.Errorf("create node offline alert [%d]: %s", node.ID, err.Error())
		return
	}
	SendAlertToChannels(ctx, s.notifChannelRepo, s.deliveryRepo, a.ID, "告警: 节点离线", msg)
	s.log.Warnf("ALERT: %s", msg)
}

// fireNodeOnlineRecovery 节点恢复在线(心跳/准入到达):解除未恢复失联告警,
// 可选落 RECOVERY 记录并通知(有未恢复告警才通知,首次上线静默)。
func (s *AgentService) fireNodeOnlineRecovery(agentID string) {
	if s.alertRepo == nil {
		return
	}
	ctx := context.Background()
	node, err := s.repo.GetByAgentID(ctx, agentID)
	if err != nil || node == nil {
		return
	}
	unresolved, _ := s.alertRepo.HasUnresolvedAlert(ctx, node.ID, "NODE_OFFLINE")
	if !unresolved {
		return
	}
	if _, err = s.alertRepo.ResolveAllByMonitor(ctx, node.ID, "NODE_OFFLINE"); err != nil {
		s.log.Warnf("resolve node offline alerts [%d]: %s", node.ID, err.Error())
		return
	}
	msg := fmt.Sprintf("节点 [%s] 已恢复在线,失联告警解除", agentDisplayName(node))
	if a, err := s.alertRepo.CreateRecovery(ctx, node.ID, 0, node.ID, "NODE_OFFLINE", msg); err != nil {
		s.log.Errorf("create node online recovery alert [%d]: %s", node.ID, err.Error())
		return
	} else {
		SendAlertToChannels(ctx, s.notifChannelRepo, s.deliveryRepo, a.ID, "恢复: 节点上线", msg)
	}
	s.log.Infof("RECOVERED: %s", msg)
}

// agentDisplayName 节点展示名:显示名 → 主机名 → agentId 兜底。
func agentDisplayName(node *ent.AgentNode) string {
	if node.Name != nil && *node.Name != "" {
		return *node.Name
	}
	if node.Hostname != nil && *node.Hostname != "" {
		return *node.Hostname
	}
	return node.AgentID
}

func formatTimeOrUnknown(t *time.Time) string {
	if t == nil {
		return "未知"
	}
	return t.Format("2006-01-02 15:04:05")
}

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
