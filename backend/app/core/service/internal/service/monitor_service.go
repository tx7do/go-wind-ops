package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/signal"
)

// MonitorService 监控规则 CRUD 门面;检查调度逻辑在 CronScheduler.tickMonitors。
type MonitorService struct {
	opsV1.UnimplementedMonitorServiceServer

	log  *log.Helper
	repo *data.MonitorRepo
}

func NewMonitorService(ctx *bootstrap.Context, repo *data.MonitorRepo) *MonitorService {
	return &MonitorService{
		log:  ctx.NewLoggerHelper("monitor/service/core-service"),
		repo: repo,
	}
}

func (s *MonitorService) ListMonitors(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMonitorResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *MonitorService) GetMonitor(ctx context.Context, req *opsV1.GetMonitorRequest) (*opsV1.Monitor, error) {
	return s.repo.Get(ctx, req)
}

func (s *MonitorService) CreateMonitor(ctx context.Context, req *opsV1.CreateMonitorRequest) (*opsV1.Monitor, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetName() == "" || req.Data.GetCron() == "" {
		return nil, opsV1.ErrorBadRequest("name/cron are required")
	}
	if _, err := cron.ParseStandard(req.Data.GetCron()); err != nil {
		return nil, opsV1.ErrorBadRequest("invalid cron expression")
	}

	// 目标类型分支:项目状态规则要求 projectId;节点指标规则要求
	// nodeId + 有效阈值(0-100];进程规则要求 nodeId + 进程名关键字
	switch req.Data.GetTargetType() {
	case "NODE_METRIC":
		if req.Data.GetNodeId() == 0 {
			return nil, opsV1.ErrorBadRequest("nodeId is required for node metric monitor")
		}
		if req.Data.GetThresholdPercent() <= 0 || req.Data.GetThresholdPercent() > 100 {
			return nil, opsV1.ErrorBadRequest("thresholdPercent must be in (0, 100]")
		}
		if mt := req.Data.GetMetricType(); mt != "" && mt != "CPU" && mt != "MEMORY" && mt != "DISK" {
			return nil, opsV1.ErrorBadRequest("invalid metricType")
		}
	case "PROCESS":
		if req.Data.GetNodeId() == 0 {
			return nil, opsV1.ErrorBadRequest("nodeId is required for process monitor")
		}
		if req.Data.GetProcessName() == "" {
			return nil, opsV1.ErrorBadRequest("processName is required for process monitor")
		}
	default:
		if req.Data.GetProjectId() == 0 {
			return nil, opsV1.ErrorBadRequest("projectId is required for project monitor")
		}
	}

	taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, opsV1.ErrorConflict("monitor name already exists")
	}

	return s.repo.Create(ctx, req, operatorIDOf(ctx))
}

func (s *MonitorService) UpdateMonitor(ctx context.Context, req *opsV1.UpdateMonitorRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if req.Data.GetCron() != "" {
		if _, err := cron.ParseStandard(req.Data.GetCron()); err != nil {
			return nil, opsV1.ErrorBadRequest("invalid cron expression")
		}
	}

	if req.Data.Name != nil {
		taken, err := s.repo.IsNameTaken(ctx, req.Data.GetName(), req.GetId())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, opsV1.ErrorConflict("monitor name already exists")
		}
	}

	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *MonitorService) DeleteMonitor(ctx context.Context, req *opsV1.DeleteMonitorRequest) (*emptypb.Empty, error) {
	if err := s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// AlertService 告警记录查询 + 人工确认恢复。
type AlertService struct {
	opsV1.UnimplementedAlertServiceServer

	log  *log.Helper
	repo *data.AlertRepo
}

func NewAlertService(ctx *bootstrap.Context, repo *data.AlertRepo) *AlertService {
	return &AlertService{
		log:  ctx.NewLoggerHelper("alert/service/core-service"),
		repo: repo,
	}
}

func (s *AlertService) ListAlerts(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAlertResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *AlertService) GetAlert(ctx context.Context, req *opsV1.GetAlertRequest) (*opsV1.Alert, error) {
	return s.repo.Get(ctx, req)
}

func (s *AlertService) ResolveAlert(ctx context.Context, req *opsV1.ResolveAlertRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Resolve(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ============================================================================
// 监控检查 + webhook 告警通知(由 CronScheduler.tickMonitors 调用)
// ============================================================================

// CheckMonitor 对单个监控规则执行一次状态检查:
// 项目状态规则走同步指令探测;节点指标规则直接读心跳快照判阈值。
// 由 CronScheduler 在规则 cron 命中时调用。
func CheckMonitor(ctx context.Context, log *log.Helper, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, monitorRepo *data.MonitorRepo, agentSvc *AgentService, monitor *ent.Monitor) {
	switch string(monitor.TargetType) {
	case "NODE_METRIC":
		CheckNodeMetricMonitor(ctx, log, alertRepo, notifChannelRepo, deliveryRepo, nodeRepo, monitorRepo, monitor)
	case "PROCESS":
		CheckProcessMonitor(ctx, log, alertRepo, notifChannelRepo, deliveryRepo, nodeRepo, monitorRepo, agentSvc, monitor)
	default:
		checkProjectMonitor(ctx, log, alertRepo, notifChannelRepo, deliveryRepo, projectRepo, nodeRepo, agentSvc, monitor)
	}
}

// alertIntervalAllowed 重复告警间隔判断:repeat_interval>0 时,距该规则
// 最近一条告警不足间隔则跳过本次新建(0=不限制,保持旧行为)。
func alertIntervalAllowed(ctx context.Context, alertRepo *data.AlertRepo, monitor *ent.Monitor) bool {
	if monitor.RepeatIntervalMinutes <= 0 {
		return true
	}
	latest, err := alertRepo.LatestAlertByMonitor(ctx, monitor.ID)
	if err != nil || latest == nil {
		return true
	}
	if latest.CreatedAt == nil {
		return true
	}
	return time.Since(*latest.CreatedAt) >= time.Duration(monitor.RepeatIntervalMinutes)*time.Minute
}

// metricDisplayName 指标类型中文名。
func metricDisplayName(metricType string) string {
	switch metricType {
	case "MEMORY":
		return "内存"
	case "DISK":
		return "磁盘"
	default:
		return "CPU"
	}
}

func ptrFloatOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// CheckNodeMetricMonitor 节点指标阈值检查:心跳快照(在线节点才有新鲜
// 指标)读 CPU/内存/磁盘百分比,连续 N 个周期 ≥ 阈值才告警(防抖),恢复时
// 自动落 RECOVERY 记录并通知。节点离线跳过(离线告警由 AgentService
// 离线扫描的 NODE_OFFLINE 告警负责)。
func CheckNodeMetricMonitor(ctx context.Context, log *log.Helper, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, nodeRepo *data.AgentNodeRepo, monitorRepo *data.MonitorRepo, monitor *ent.Monitor) {
	nodeID := monitor.NodeID
	if nodeID == 0 {
		return
	}
	node, err := nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		log.Warnf("node metric monitor [%d] node [%d] not found, skip", monitor.ID, nodeID)
		return
	}
	if node.GetStatus() != opsV1.AgentNodeStatus_ONLINE {
		return
	}

	var value *float64
	nodeName := node.GetName()
	if nodeName == "" {
		nodeName = node.GetHostname()
	}
	switch string(monitor.MetricType) {
	case "MEMORY":
		value = node.MemoryUsedPercent
	case "DISK":
		value = node.DiskUsedPercent
	default:
		value = node.CpuPercent
	}

	hit := value != nil && *value >= monitor.ThresholdPercent
	if hit {
		hits := monitor.ConsecutiveHits + 1
		if err = monitorRepo.UpdateConsecutiveHits(ctx, monitor.ID, hits); err != nil {
			log.Warnf("update monitor hits [%d]: %s", monitor.ID, err.Error())
		}
		if hits < monitor.ConsecutiveCount {
			return
		}

		already, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
		if already {
			return
		}
		if !alertIntervalAllowed(ctx, alertRepo, monitor) {
			return
		}

		metricName := metricDisplayName(string(monitor.MetricType))
		msg := fmt.Sprintf("节点 [%s] %s使用率 %.1f%%,连续 %d 次超过阈值 %.1f%%",
			nodeName, metricName, *value, hits, monitor.ThresholdPercent)
		alertID := uint32(0)
		if a, err := alertRepo.Create(ctx, monitor.ID, 0, nodeID, "MONITOR", "WARNING", msg, false); err != nil {
			log.Errorf("create node metric alert failed: %s", err.Error())
		} else {
			alertID = a.ID
			if monitor.WebhookURL != nil && *monitor.WebhookURL != "" {
				notifyWebhook(ctx, *monitor.WebhookURL, map[string]any{
					"alert_id":    a.ID,
					"monitor":     monitor.Name,
					"node":        nodeName,
					"node_id":     nodeID,
					"metric":      string(monitor.MetricType),
					"value":       *value,
					"threshold":   monitor.ThresholdPercent,
					"level":       "WARNING",
					"message":     msg,
					"occurred_at": time.Now().Format(time.RFC3339),
				})
				_ = alertRepo.MarkNotified(ctx, a.ID)
			}
		}
		SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, alertID, "告警: "+monitor.Name, msg)
		log.Warnf("ALERT: %s", msg)
		return
	}

	// 未命中:计数归零;如有未恢复告警则恢复 + RECOVERY 记录 + 通知
	if monitor.ConsecutiveHits != 0 {
		if err = monitorRepo.UpdateConsecutiveHits(ctx, monitor.ID, 0); err != nil {
			log.Warnf("reset monitor hits [%d]: %s", monitor.ID, err.Error())
		}
	}
	unresolved, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
	if !unresolved {
		return
	}
	if _, err = alertRepo.ResolveAllByMonitor(ctx, monitor.ID); err != nil {
		log.Warnf("resolve node metric alerts [%d]: %s", monitor.ID, err.Error())
	}
	if monitor.NotifyOnRecovery {
		metricName := metricDisplayName(string(monitor.MetricType))
		msg := fmt.Sprintf("节点 [%s] %s使用率已回落至 %.1f%%,低于阈值 %.1f%%,告警恢复",
			nodeName, metricName, ptrFloatOrZero(value), monitor.ThresholdPercent)
		if a, err := alertRepo.CreateRecovery(ctx, monitor.ID, 0, nodeID, "MONITOR", msg); err != nil {
			log.Errorf("create recovery alert failed: %s", err.Error())
		} else {
			SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, a.ID, "恢复: "+monitor.Name, msg)
		}
		log.Infof("RECOVERED: %s", msg)
	}
}

// CheckProcessMonitor 进程存在性检查:经 process_list 同步指令在目标节点
// 按关键字(大小写不敏感包含)找进程,找不到视为异常命中(防抖与节点指标
// 规则同款);重新发现进程时统一 resolve + RECOVERY 记录 + 通知。探测失败
// (节点离线/超时)不告警——节点失联由 NODE_OFFLINE 告警负责。
func CheckProcessMonitor(ctx context.Context, log *log.Helper, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, nodeRepo *data.AgentNodeRepo, monitorRepo *data.MonitorRepo, agentSvc *AgentService, monitor *ent.Monitor) {
	nodeID := monitor.NodeID
	if nodeID == 0 || monitor.ProcessName == "" {
		return
	}
	node, err := nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		log.Warnf("process monitor [%d] node [%d] not found, skip", monitor.ID, nodeID)
		return
	}
	nodeName := node.GetName()
	if nodeName == "" {
		nodeName = node.GetHostname()
	}

	res, err := agentSvc.PublishCommandSync(ctx, node.GetId(), node.GetAgentId(),
		signal.CommandActionProcessList,
		&signal.ProcessListPayload{Name: monitor.ProcessName, Limit: 50},
		0, 15*time.Second)
	if err != nil {
		log.Warnf("process monitor [%d] probe failed: %s", monitor.ID, err.Error())
		return
	}
	if !res.Success {
		log.Warnf("process monitor [%d] probe returned failure: %s", monitor.ID, res.Error)
		return
	}
	var out signal.ProcessListResult
	_ = json.Unmarshal(res.Data, &out)
	found := len(out.Processes) > 0

	if !found {
		hits := monitor.ConsecutiveHits + 1
		if err = monitorRepo.UpdateConsecutiveHits(ctx, monitor.ID, hits); err != nil {
			log.Warnf("update monitor hits [%d]: %s", monitor.ID, err.Error())
		}
		if hits < monitor.ConsecutiveCount {
			return
		}

		already, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
		if already {
			return
		}
		if !alertIntervalAllowed(ctx, alertRepo, monitor) {
			return
		}

		msg := fmt.Sprintf("节点 [%s] 未发现进程 [%s](匹配 0 个,可能已退出)", nodeName, monitor.ProcessName)
		alertID := uint32(0)
		if a, err := alertRepo.Create(ctx, monitor.ID, 0, nodeID, "MONITOR", "ERROR", msg, false); err != nil {
			log.Errorf("create process alert failed: %s", err.Error())
		} else {
			alertID = a.ID
			if monitor.WebhookURL != nil && *monitor.WebhookURL != "" {
				notifyWebhook(ctx, *monitor.WebhookURL, map[string]any{
					"alert_id":    a.ID,
					"monitor":     monitor.Name,
					"node":        nodeName,
					"node_id":     nodeID,
					"process":     monitor.ProcessName,
					"level":       "ERROR",
					"message":     msg,
					"occurred_at": time.Now().Format(time.RFC3339),
				})
				_ = alertRepo.MarkNotified(ctx, a.ID)
			}
		}
		SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, alertID, "告警: "+monitor.Name, msg)
		log.Warnf("ALERT: %s", msg)
		return
	}

	// 进程在:计数归零;有未恢复告警则恢复 + RECOVERY 记录 + 通知
	if monitor.ConsecutiveHits != 0 {
		if err = monitorRepo.UpdateConsecutiveHits(ctx, monitor.ID, 0); err != nil {
			log.Warnf("reset monitor hits [%d]: %s", monitor.ID, err.Error())
		}
	}
	unresolved, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
	if !unresolved {
		return
	}
	if _, err = alertRepo.ResolveAllByMonitor(ctx, monitor.ID); err != nil {
		log.Warnf("resolve process alerts [%d]: %s", monitor.ID, err.Error())
	}
	if monitor.NotifyOnRecovery {
		msg := fmt.Sprintf("节点 [%s] 重新发现进程 [%s](匹配 %d 个),告警恢复", nodeName, monitor.ProcessName, out.Total)
		if a, err := alertRepo.CreateRecovery(ctx, monitor.ID, 0, nodeID, "MONITOR", msg); err != nil {
			log.Errorf("create recovery alert failed: %s", err.Error())
		} else {
			SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, a.ID, "恢复: "+monitor.Name, msg)
		}
		log.Infof("RECOVERED: %s", msg)
	}
}

// checkProjectMonitor 项目状态规则检查:同步指令探测项目状态 → 异常按
// 重复间隔落告警 → 可选自动重启 → 渠道外呼;恢复正常时统一 resolve +
// RECOVERY 记录 + 恢复通知(与节点指标规则对齐)。
func checkProjectMonitor(ctx context.Context, log *log.Helper, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, agentSvc *AgentService, monitor *ent.Monitor) {
	projectID := monitor.ProjectID

	project, err := projectRepo.Get(ctx, &opsV1.GetProjectRequest{Id: &projectID})
	if err != nil {
		log.Warnf("monitor [%d] project [%d] not found, skip", monitor.ID, projectID)
		return
	}
	nodeID := project.GetNodeId()
	node, err := nodeRepo.Get(ctx, &opsV1.GetAgentNodeRequest{Id: &nodeID})
	if err != nil {
		log.Warnf("monitor [%d] project [%d] node not found, skip", monitor.ID, projectID)
		return
	}

	// 同步指令探测状态(10s 超时)
	res, err := agentSvc.PublishCommandSync(ctx, node.GetId(), node.GetAgentId(),
		"project_status",
		map[string]any{"project_id": project.GetId(), "dir": project.GetProjectDir()},
		0, 10*time.Second)
	if err != nil {
		log.Warnf("monitor [%d] status probe failed: %s", monitor.ID, err.Error())
		return // 探测失败不告警(节点离线由离线扫描负责)
	}
	if !res.Success {
		log.Warnf("monitor [%d] status probe returned failure: %s", monitor.ID, res.Error)
		return
	}

	// 解析状态
	var status struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(res.Data, &status)
	if status.Status == "RUNNING" {
		// ── 正常运行:统一恢复语义(与节点指标规则对齐)──
		unresolved, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
		if !unresolved {
			return
		}
		if _, err := alertRepo.ResolveAllByMonitor(ctx, monitor.ID); err != nil {
			log.Warnf("resolve project alerts [%d]: %s", monitor.ID, err.Error())
		}
		if monitor.NotifyOnRecovery {
			msg := fmt.Sprintf("项目 [%s] 已恢复 RUNNING,告警关闭", project.GetName())
			if a, err := alertRepo.CreateRecovery(ctx, monitor.ID, projectID, 0, "MONITOR", msg); err != nil {
				log.Errorf("create recovery alert failed: %s", err.Error())
			} else {
				SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, a.ID, "恢复: "+monitor.Name, msg)
				log.Infof("RECOVERED: %s", msg)
			}
		}
		return
	}

	// ── 异常:状态不是 RUNNING ──
	if !alertIntervalAllowed(ctx, alertRepo, monitor) {
		return // 距上次告警不足重复间隔,保持既有未恢复告警即可
	}
	already, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
	if !already {
		msg := fmt.Sprintf("项目 [%s] 状态异常: %s", project.GetName(), status.Status)
		alertID := uint32(0)
		if a, err := alertRepo.Create(ctx, monitor.ID, projectID, 0, "MONITOR", "ERROR", msg, false); err != nil {
			log.Errorf("create alert failed: %s", err.Error())
		} else {
			alertID = a.ID
			if monitor.WebhookURL != nil && *monitor.WebhookURL != "" {
				notifyWebhook(ctx, *monitor.WebhookURL, map[string]any{
					"alert_id":    a.ID,
					"monitor":     monitor.Name,
					"project":     project.GetName(),
					"level":       "ERROR",
					"message":     msg,
					"occurred_at": time.Now().Format(time.RFC3339),
				})
				_ = alertRepo.MarkNotified(ctx, a.ID)
			}
		}
		SendAlertToChannels(ctx, notifChannelRepo, deliveryRepo, alertID, "告警: "+monitor.Name, msg)
		log.Warnf("ALERT: %s", msg)
	}

	// 自动重启
	if monitor.AutoRestart {
		log.Infof("monitor [%d] auto restarting project [%d]", monitor.ID, projectID)
		_, _ = agentSvc.PublishCommand(ctx, node.GetId(), node.GetAgentId(),
			"project_restart",
			map[string]any{"project_id": project.GetId(), "dir": project.GetProjectDir()},
			0)
	}
}

// notifyWebhook POST JSON 告警通知(5s 超时,失败仅记日志不重试)。
func notifyWebhook(ctx context.Context, url string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[webhook] notify %s failed: %v\n", url, err)
		return
	}
	defer resp.Body.Close()
}

