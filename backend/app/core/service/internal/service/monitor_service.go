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
	if req.Data.GetName() == "" || req.Data.GetProjectId() == 0 || req.Data.GetCron() == "" {
		return nil, opsV1.ErrorBadRequest("name/projectId/cron are required")
	}
	if _, err := cron.ParseStandard(req.Data.GetCron()); err != nil {
		return nil, opsV1.ErrorBadRequest("invalid cron expression")
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
// 同步指令探测项目状态 → 异常则去重落告警 → 可选自动重启 → 可选 webhook 外呼。
// 由 CronScheduler 在规则 cron 命中时调用。
func CheckMonitor(ctx context.Context, log *log.Helper, alertRepo *data.AlertRepo, notifChannelRepo *data.NotificationChannelRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, agentSvc *AgentService, monitor *ent.Monitor) {
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
		return // 项目正常运行,无需告警;同时恢复已有的未恢复告警
	}
	_, _ = alertRepo.ResolveAllByMonitor(ctx, monitor.ID)

	// ── 异常:状态不是 RUNNING ──
	already, _ := alertRepo.HasUnresolvedAlert(ctx, monitor.ID)
	if !already {
		msg := fmt.Sprintf("项目 [%s] 状态异常: %s", project.GetName(), status.Status)
		if a, err := alertRepo.Create(ctx, monitor.ID, projectID, "ERROR", msg, false); err != nil {
			log.Errorf("create alert failed: %s", err.Error())
		} else if monitor.WebhookURL != nil && *monitor.WebhookURL != "" {
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
		SendAlertToWebhooks(ctx, notifChannelRepo, "告警: "+monitor.Name, msg)
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

