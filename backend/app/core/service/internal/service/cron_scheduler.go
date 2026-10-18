package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	"go-wind-ops/app/core/service/internal/data"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// cronSweepInterval 调度扫描周期;cron 表达式最小粒度为分钟,30s 保证
// 不会跨过触发点太久,也不会重复触发(锚点 CAS 去重)。
const cronSweepInterval = 30 * time.Second

// CronScheduler 定时调度器:每 30s 扫描定时构建/脚本/监控规则,命中即触发。
//
// 去重:构建/脚本用 last_cron_at 锚点 + CAS 推进(多副本安全);
// 监控检查只读幂等,重复无害。
type CronScheduler struct {
	ctx           *bootstrap.Context
	buildRepo     *data.BuildRepo
	scriptRepo    *data.ScriptRepo
	monitorRepo   *data.MonitorRepo
	alertRepo     *data.AlertRepo
	projectRepo   *data.ProjectRepo
	nodeRepo      *data.AgentNodeRepo
	buildService  *BuildService
	scriptService *ScriptService
	agentService  *AgentService
	log           *log.Helper
}

func NewCronScheduler(ctx *bootstrap.Context, buildRepo *data.BuildRepo, scriptRepo *data.ScriptRepo, monitorRepo *data.MonitorRepo, alertRepo *data.AlertRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, buildService *BuildService, scriptService *ScriptService, agentService *AgentService) *CronScheduler {
	return &CronScheduler{
		ctx:           ctx,
		buildRepo:     buildRepo,
		scriptRepo:    scriptRepo,
		monitorRepo:   monitorRepo,
		alertRepo:     alertRepo,
		projectRepo:   projectRepo,
		nodeRepo:      nodeRepo,
		buildService:  buildService,
		scriptService: scriptService,
		agentService:  agentService,
		log:           ctx.NewLoggerHelper("cron-scheduler/service/core-service"),
	}
}

// Start 启动调度循环。
func (s *CronScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cronSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tickBuilds(ctx)
				s.tickScripts(ctx)
				s.tickMonitors(ctx)
			}
		}
	}()
	s.log.Info("cron scheduler started (30s tick)")
}

// tickBuilds 扫描定时构建。
func (s *CronScheduler) tickBuilds(ctx context.Context) {
	resp, err := s.buildRepo.List(ctx, allPaging())
	if err != nil {
		return
	}
	now := time.Now()
	for _, b := range resp.GetItems() {
		spec := b.GetCron()
		if spec == "" {
			continue
		}
		schedule, err := cron.ParseStandard(spec)
		if err != nil {
			continue
		}
		last := b.GetLastCronAt().AsTime()
		if last.IsZero() {
			last = now
		}
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.buildRepo.AdvanceCronAnchor(ctx, b.GetId(), last, now)
		if err != nil || !ok {
			continue
		}
		s.log.Infof("cron build triggered (build=%d no=%d spec=%s)", b.GetId(), b.GetBuildNo()+1, spec)
		if _, err = s.buildService.TriggerBuild(ctx, &opsV1.TriggerBuildRequest{Id: b.Id}); err != nil {
			s.log.Errorf("cron build trigger failed (build=%d): %s", b.GetId(), err.Error())
		}
	}
}

// tickScripts 扫描定时脚本。
func (s *CronScheduler) tickScripts(ctx context.Context) {
	resp, err := s.scriptRepo.List(ctx, allPaging())
	if err != nil {
		return
	}
	now := time.Now()
	for _, sc := range resp.GetItems() {
		spec := sc.GetCron()
		nodeID := sc.GetCronNodeId()
		if spec == "" || nodeID == 0 {
			continue
		}
		schedule, err := cron.ParseStandard(spec)
		if err != nil {
			continue
		}
		last := sc.GetLastCronAt().AsTime()
		if last.IsZero() {
			last = now
		}
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.scriptRepo.AdvanceCronAnchor(ctx, sc.GetId(), last, now)
		if err != nil || !ok {
			continue
		}
		s.log.Infof("cron script triggered (script=%d node=%d spec=%s)", sc.GetId(), nodeID, spec)
		if _, err = s.scriptService.ExecuteScript(ctx, &opsV1.ExecuteScriptRequest{
			Id:     sc.Id,
			NodeId: &nodeID,
		}); err != nil {
			s.log.Errorf("cron script execute failed (script=%d): %s", sc.GetId(), err.Error())
		}
	}
}

// tickMonitors 扫描监控规则:cron 命中即对项目做状态探测(只读,重复无害)。
func (s *CronScheduler) tickMonitors(ctx context.Context) {
	monitors, err := s.monitorRepo.ListEnabled(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	for _, m := range monitors {
		schedule, err := cron.ParseStandard(m.Cron)
		if err != nil {
			continue
		}
		if !cronHitMinute(schedule, now) {
			continue
		}
		CheckMonitor(ctx, s.log, s.alertRepo, s.projectRepo, s.nodeRepo, s.agentService, m)
	}
}

// cronHitMinute 判断 cron 调度在当前分钟内是否命中。
func cronHitMinute(schedule cron.Schedule, now time.Time) bool {
	next := schedule.Next(now.Add(-time.Minute))
	return !next.After(now)
}

func allPaging() *paginationV1.PagingRequest {
	return &paginationV1.PagingRequest{
		NoPaging: trans.Ptr(true),
	}
}
