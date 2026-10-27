package service

import (
	"context"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/minio/minio-go/v7"
	"github.com/robfig/cron/v3"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/app/core/service/internal/data"
	appViewer "go-wind-ops/pkg/entgo/viewer"
	"go-wind-ops/pkg/oss"
)

// cronSweepInterval 调度扫描周期;cron 表达式最小粒度为分钟,30s 保证
// 不会跨过触发点太久,也不会重复触发(锚点 CAS 去重)。
const cronSweepInterval = 30 * time.Second

// CronScheduler 定时调度器:每 30s 扫描定时构建/脚本/监控/备份任务,命中即触发。
//
// 去重:构建/脚本/监控/备份全部用锚点 CAS 推进(多副本安全)——构建/脚本/
// 备份锚在 last_cron_at,监控检查锚在 last_check_at;同一规则同一周期只有
// 抢到锚点的副本执行,探测指令、防抖计数与告警创建随之单份。
type CronScheduler struct {
	ctx                *bootstrap.Context
	buildRepo          *data.BuildRepo
	scriptRepo         *data.ScriptRepo
	monitorRepo        *data.MonitorRepo
	alertRepo          *data.AlertRepo
	projectRepo        *data.ProjectRepo
	nodeRepo           *data.AgentNodeRepo
	backupTaskRepo     *data.BackupTaskRepo
	buildService       *BuildService
	scriptService      *ScriptService
	backupService      *BackupService
	agentService       *AgentService
	notifChannelRepo   *data.NotificationChannelRepo
	deliveryRepo       *data.NotificationDeliveryRepo
	maintenanceRepo   *data.MaintenanceWindowRepo
	approvalRepo      *data.ApprovalRepo
	budgetRepo         *data.CloudBudgetRepo
	budgetService      *CloudBudgetService
	certRepo           *data.CertRepo
	certService        *CertService
	logSnapshotService *LogSnapshotService
	historyRepo        *data.BuildHistoryRepo
	pipelineRunRepo    *data.PipelineRunRepo
	ossClient          *oss.MinIOClient
	auditRetentionRepo *data.AuditRetentionRepo

	lastRetentionSweep      time.Time
	lastAuditRetentionSweep time.Time
	lastApprovalExpirySweep time.Time
	lastOrphanSweep         time.Time
	lastBudgetCheck         time.Time
	lastCertCheck           time.Time
	lastLogSnapshot         time.Time
	lastTransientSweep      time.Time
	log                     *log.Helper
}

func NewCronScheduler(ctx *bootstrap.Context, buildRepo *data.BuildRepo, scriptRepo *data.ScriptRepo, monitorRepo *data.MonitorRepo, alertRepo *data.AlertRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, backupTaskRepo *data.BackupTaskRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, maintenanceRepo *data.MaintenanceWindowRepo, approvalRepo *data.ApprovalRepo, budgetRepo *data.CloudBudgetRepo, budgetService *CloudBudgetService, certRepo *data.CertRepo, certService *CertService, logSnapshotService *LogSnapshotService, historyRepo *data.BuildHistoryRepo, pipelineRunRepo *data.PipelineRunRepo, ossClient *oss.MinIOClient, buildService *BuildService, scriptService *ScriptService, backupService *BackupService, agentService *AgentService, auditRetentionRepo *data.AuditRetentionRepo) *CronScheduler {
	return &CronScheduler{
		ctx:                ctx,
		buildRepo:          buildRepo,
		scriptRepo:         scriptRepo,
		monitorRepo:        monitorRepo,
		alertRepo:          alertRepo,
		projectRepo:        projectRepo,
		nodeRepo:           nodeRepo,
		backupTaskRepo:     backupTaskRepo,
		notifChannelRepo:   notifChannelRepo,
		deliveryRepo:       deliveryRepo,
		maintenanceRepo:   maintenanceRepo,
		approvalRepo:      approvalRepo,
		budgetRepo:         budgetRepo,
		budgetService:      budgetService,
		certRepo:           certRepo,
		certService:        certService,
		logSnapshotService: logSnapshotService,
		historyRepo:        historyRepo,
		pipelineRunRepo:    pipelineRunRepo,
		ossClient:          ossClient,
		buildService:       buildService,
		scriptService:      scriptService,
		backupService:      backupService,
		agentService:       agentService,
		auditRetentionRepo: auditRetentionRepo,
		log:                ctx.NewLoggerHelper("cron-scheduler/service/core-service"),
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
				s.runTick(ctx, "builds", s.tickBuilds)
				s.runTick(ctx, "scripts", s.tickScripts)
				s.runTick(ctx, "monitors", s.tickMonitors)
				s.runTick(ctx, "backups", s.tickBackups)
				s.runTick(ctx, "orphan-runs", s.tickOrphanRuns)
				s.runTick(ctx, "budgets", s.tickBudgets)
				s.runTick(ctx, "certs", s.tickCerts)
				s.runTick(ctx, "log-snapshots", s.tickLogSnapshots)
				s.runTick(ctx, "retention", s.tickRetention)
				s.runTick(ctx, "audit-retention", s.tickAuditRetention)
				s.runTick(ctx, "transient-objects", s.tickTransientObjects)
				s.runTick(ctx, "approval-expiry", s.tickApprovalExpiry)
			}
		}
	}()
	s.log.Info("cron scheduler started (30s tick)")
}

// runTick 单个 tick 的 panic 隔离:调度循环是单 goroutine 顺序执行,任一
// tick 恐慌会沿调用栈炸掉整个 core 进程。这里兜住并记日志,只损失本轮该
// 任务,下一周期照常执行。
func (s *CronScheduler) runTick(ctx context.Context, name string, fn func(ctx context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Errorf("cron tick [%s] panicked: %v\n%s", name, r, debug.Stack())
		}
	}()
	fn(ctx)
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
		if b.GetLastCronAt() == nil {
			// 新任务的空锚点先落当前时间(不补跑),从下一个 cron 周期起
			// 正常触发;NULL 直接走 CAS 会因 EQ(NULL) 永远匹配不上而永不触发。
			if ok, _ := s.buildRepo.AdvanceCronAnchorFromNull(ctx, b.GetId(), now); ok {
				s.log.Infof("cron build anchor initialized (build=%d spec=%s)", b.GetId(), spec)
			}
			continue
		}
		last := b.GetLastCronAt().AsTime()
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.buildRepo.AdvanceCronAnchor(ctx, b.GetId(), last, now)
		if err != nil || !ok {
			continue
		}
		s.log.Infof("cron build triggered (build=%d no=%d spec=%s)", b.GetId(), b.GetBuildNo()+1, spec)
		// 租户归因:定时触发的构建归属配置作者的租户(后台路径默认落 tenant 0)。
		buildCtx := ctx
		if entity, gerr := s.buildRepo.GetEntity(ctx, b.GetId()); gerr == nil && entity != nil {
			buildCtx = tenantScopedCtx(ctx, entity.TenantID)
		}
		if _, err = s.buildService.TriggerBuild(buildCtx, &opsV1.TriggerBuildRequest{Id: b.GetId()}); err != nil {
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
		if sc.GetLastCronAt() == nil {
			// 同 tickBuilds:空锚点先落锚,避免 NULL 永不触发。
			if ok, _ := s.scriptRepo.AdvanceCronAnchorFromNull(ctx, sc.GetId(), now); ok {
				s.log.Infof("cron script anchor initialized (script=%d spec=%s)", sc.GetId(), spec)
			}
			continue
		}
		last := sc.GetLastCronAt().AsTime()
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.scriptRepo.AdvanceCronAnchor(ctx, sc.GetId(), last, now)
		if err != nil || !ok {
			continue
		}
		s.log.Infof("cron script triggered (script=%d node=%d spec=%s)", sc.GetId(), nodeID, spec)
		// 租户归因:定时触发的脚本执行/指令台账归属脚本作者的租户
		// (后台路径无操作者上下文,默认会落 tenant 0)。
		execCtx := ctx
		if entity, gerr := s.scriptRepo.GetEntity(ctx, sc.GetId()); gerr == nil && entity != nil {
			execCtx = tenantScopedCtx(ctx, entity.TenantID)
		}
		// 自动化链路:定时脚本入库时已被作者审阅,显式确认通过危险命令门槛。
		if _, err = s.scriptService.ExecuteScript(execCtx, &opsV1.ExecuteScriptRequest{
			Id:               sc.GetId(),
			NodeId:           &nodeID,
			ConfirmDangerous: trans.Ptr(true),
		}); err != nil {
			s.log.Errorf("cron script execute failed (script=%d): %s", sc.GetId(), err.Error())
		}
	}
}

// tickMonitors 扫描监控规则:锚点 CAS 去重(last_check_at),同一规则同一
// 周期只有抢到锚点的副本执行检查——检查并非"只读幂等":它会创建告警、
// 外呼 webhook、推进防抖计数,多副本双跑会双发告警,必须单份执行。
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
		if m.LastCheckAt == nil {
			// 新规则的空锚点先落当前时间(不补跑),从下一个 cron 周期起
			// 正常检查;NULL 直接走 CAS 会因 EQ(NULL) 永远匹配不上而永不触发。
			if ok, _ := s.monitorRepo.AdvanceCheckAnchorFromNull(ctx, m.ID, now); ok {
				s.log.Infof("monitor check anchor initialized (monitor=%d spec=%s)", m.ID, m.Cron)
			}
			continue
		}
		last := *m.LastCheckAt
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.monitorRepo.AdvanceCheckAnchor(ctx, m.ID, last, now)
		if err != nil || !ok {
			continue
		}
		CheckMonitor(ctx, s.log, s.alertRepo, s.notifChannelRepo, s.deliveryRepo, s.projectRepo, s.nodeRepo, s.monitorRepo, s.agentService, s.maintenanceRepo, m)
	}
}

// orphanSweepInterval 孤儿构建/流水线清扫周期(低频,每小时一次)。
const orphanSweepInterval = time.Hour

// tickOrphanRuns 周期清扫孤儿运行记录:runner 副本崩溃后无法自 Clean 的
// PENDING/RUNNING 构建历史与流水线 run,按 updated_at 停更超阈值兜底定案
// FAILED(启动清扫只管本实例,跨实例的遗留归这里)。条件更新幂等,多副本
// 并发执行恰一方生效。
func (s *CronScheduler) tickOrphanRuns(ctx context.Context) {
	if s.historyRepo == nil {
		return
	}
	if time.Since(s.lastOrphanSweep) < orphanSweepInterval {
		return
	}
	s.lastOrphanSweep = time.Now()
	if n, err := s.historyRepo.FinalizeStaleInterrupted(ctx); err != nil {
		s.log.Warnf("orphan build sweep: %s", err.Error())
	} else if n > 0 {
		s.log.Infof("orphan build sweep: finalized %d record(s)", n)
	}
	if s.pipelineRunRepo != nil {
		if n, err := s.pipelineRunRepo.FinalizeStaleInterrupted(ctx); err != nil {
			s.log.Warnf("orphan pipeline sweep: %s", err.Error())
		} else if n > 0 {
			s.log.Infof("orphan pipeline sweep: finalized %d run(s)", n)
		}
	}
}

// tickBackups 扫描定时备份任务(锚点 CAS 去重)。新建任务的空锚点先落
// 当前时间(不补跑),从下一个 cron 周期起正常触发。
func (s *CronScheduler) tickBackups(ctx context.Context) {
	if s.backupTaskRepo == nil || s.backupService == nil {
		return
	}
	tasks, err := s.backupTaskRepo.ListEnabledWithCron(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	for _, task := range tasks {
		spec := task.Cron
		if spec == "" {
			continue
		}
		schedule, err := cron.ParseStandard(spec)
		if err != nil {
			continue
		}
		if task.LastCronAt == nil {
			// 首次发现:仅落锚,等到下一个周期再触发
			if ok, _ := s.backupTaskRepo.AdvanceCronAnchorFromNull(ctx, task.ID, now); ok {
				s.log.Infof("cron backup anchor initialized (task=%d spec=%s)", task.ID, spec)
			}
			continue
		}
		last := task.LastCronAt.In(time.Local)
		next := schedule.Next(last)
		if next.After(now) {
			continue
		}
		ok, err := s.backupTaskRepo.AdvanceCronAnchor(ctx, task.ID, last, now)
		if err != nil || !ok {
			continue
		}
		s.log.Infof("cron backup triggered (task=%d spec=%s)", task.ID, spec)
		if _, err = s.backupService.TriggerBackup(ctx, &opsV1.TriggerBackupRequest{Id: &task.ID}); err != nil {
			s.log.Errorf("cron backup trigger failed (task=%d): %s", task.ID, err.Error())
		}
	}
}

func allPaging() *paginationV1.PagingRequest {
	return &paginationV1.PagingRequest{
		NoPaging: trans.Ptr(true),
	}
}

// transientObjectTTL 瞬态对象存活期:文件管理拉取(agent-files/)与配置
// 下发中转(config-templates/)都是一次性传输,超期即清。
const transientObjectTTL = 24 * time.Hour

var transientObjectPrefixes = []string{"agent-files/", "config-templates/"}

// tickTransientObjects 清理瞬态中转对象(每小时一次)。
func (s *CronScheduler) tickTransientObjects(ctx context.Context) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil {
		return
	}
	if time.Since(s.lastTransientSweep) < retentionSweepInterval {
		return
	}
	s.lastTransientSweep = time.Now()
	deadline := time.Now().Add(-transientObjectTTL)
	removed := 0
	for _, prefix := range transientObjectPrefixes {
		objects := s.ossClient.GetClient().ListObjects(ctx, oss.BucketFiles, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		})
		for obj := range objects {
			if obj.Err != nil || obj.LastModified.After(deadline) {
				continue
			}
			if err := s.ossClient.GetClient().RemoveObject(ctx, oss.BucketFiles, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				s.log.Warnf("transient object cleanup remove %s: %s", obj.Key, err.Error())
				continue
			}
			removed++
		}
	}
	if removed > 0 {
		s.log.Infof("transient object cleanup: removed %d object(s)", removed)
	}
}

// retentionSweepInterval 产物保留策略清扫周期(低频重活,每小时一次)。
const retentionSweepInterval = time.Hour

// auditRetentionSweepInterval 审计留存清扫周期(低频重活,24h 一次)。
const auditRetentionSweepInterval = 24 * time.Hour

// tickAuditRetention 审计日志留存清扫:AUDIT_RETENTION_DAYS 配置保留天数
// (默认 90;0/负值=不启用),删除五类审计表中早于留存期的行。
// tickApprovalExpiry 审批到期扫描:节拍 30s,10 分钟节流(PENDING 超时
// 分钟级精度足够)。repo 为空直接返回(wiring 未注入时不启用)。
func (s *CronScheduler) tickApprovalExpiry(ctx context.Context) {
	if s.approvalRepo == nil {
		return
	}
	if time.Since(s.lastApprovalExpirySweep) < 10*time.Minute {
		return
	}
	s.lastApprovalExpirySweep = time.Now()

	n, err := s.approvalRepo.ExpireStale(ctx, time.Now())
	if err != nil {
		s.log.Errorf("approval expiry sweep failed: %s", err.Error())
		return
	}
	if n > 0 {
		s.log.Infof("approval expiry: %d pending approval(s) expired", n)
	}
}

func (s *CronScheduler) tickAuditRetention(ctx context.Context) {
	if s.auditRetentionRepo == nil {
		return
	}
	if time.Since(s.lastAuditRetentionSweep) < auditRetentionSweepInterval {
		return
	}
	days := 0
	if v := strings.TrimSpace(os.Getenv("AUDIT_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	if days <= 0 {
		return
	}
	s.lastAuditRetentionSweep = time.Now()
	cutoff := time.Now().AddDate(0, 0, -days)
	if n, err := s.auditRetentionRepo.DeleteOlderThan(ctx, cutoff, 1000); err != nil {
		s.log.Warnf("audit retention sweep failed: %s", err.Error())
	} else if n > 0 {
		s.log.Infof("audit retention sweep: removed %d audit row(s) older than %dd", n, days)
	}
}

// tickRetention 产物保留策略清扫:对配置了 artifactRetention 的构建,
// 删除保留数量之外的历史产物对象(MinIO)并清空记录指针。
func (s *CronScheduler) tickRetention(ctx context.Context) {
	if s.ossClient == nil || s.ossClient.GetClient() == nil || s.historyRepo == nil {
		return
	}
	if time.Since(s.lastRetentionSweep) < retentionSweepInterval {
		return
	}
	s.lastRetentionSweep = time.Now()

	resp, err := s.buildRepo.List(ctx, &paginationV1.PagingRequest{NoPaging: boolPtr(true)})
	if err != nil {
		s.log.Warnf("retention sweep list builds: %s", err.Error())
		return
	}

	var removed int
	for _, b := range resp.GetItems() {
		keep := b.GetArtifactRetention()
		if keep == 0 {
			continue
		}
		excess, err := s.historyRepo.ListArtifactHoldersByBuild(ctx, b.GetId(), keep)
		if err != nil {
			s.log.Warnf("retention sweep list histories (build=%d): %s", b.GetId(), err.Error())
			continue
		}
		for _, h := range excess {
			if h.ArtifactObjectKey == nil || *h.ArtifactObjectKey == "" {
				continue
			}
			bucket := ""
			if h.ArtifactBucket != nil {
				bucket = *h.ArtifactBucket
			}
			if err = s.ossClient.GetClient().RemoveObject(ctx, bucket, *h.ArtifactObjectKey, minio.RemoveObjectOptions{}); err != nil {
				s.log.Warnf("retention sweep remove object (history=%d): %s", h.ID, err.Error())
				continue
			}
			if err = s.historyRepo.ClearArtifact(ctx, h.ID); err != nil {
				s.log.Warnf("retention sweep clear record (history=%d): %s", h.ID, err.Error())
				continue
			}
			removed++
		}
	}
	if removed > 0 {
		s.log.Infof("retention sweep done: removed %d artifacts", removed)
	}
}

// budgetCheckInterval 预算/证书检查节流(低频任务,每 30 分钟一轮)。
const budgetCheckInterval = 30 * time.Minute

// tickBudgets 云成本预算检查:当前月份预算比对台账成本,阈值/环比突增告警。
func (s *CronScheduler) tickBudgets(ctx context.Context) {
	if s.budgetService == nil {
		return
	}
	if time.Since(s.lastBudgetCheck) < budgetCheckInterval {
		return
	}
	s.lastBudgetCheck = time.Now()
	if err := s.budgetService.RunBudgetCheck(ctx, false); err != nil {
		s.log.Warnf("budget check: %s", err.Error())
	}
}

// logSnapshotIntervalMinutes 日志快照定时采集间隔(分钟),环境变量
// LOG_SNAPSHOT_INTERVAL_MINUTES 配置,0(默认)=关闭定时仅手动。
var logSnapshotIntervalMinutes = func() int {
	if v := strings.TrimSpace(os.Getenv("LOG_SNAPSHOT_INTERVAL_MINUTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}()

// tickLogSnapshots 定时采集:全部项目逐个采集(trigger=SCHEDULED),单个
// 失败不影响其余;整循环放 goroutine 避免拖长 30s tick。
func (s *CronScheduler) tickLogSnapshots(ctx context.Context) {
	if s.logSnapshotService == nil || logSnapshotIntervalMinutes <= 0 {
		return
	}
	if time.Since(s.lastLogSnapshot) < time.Duration(logSnapshotIntervalMinutes)*time.Minute {
		return
	}
	s.lastLogSnapshot = time.Now()
	go func() {
		// 扫描涉及大量已挂租户隐私的表(脚本锚点推进/监控防抖/证书探测...),
		// 先注入系统 viewer;租户归因由各 tick 内 tenantScopedCtx 覆盖。
		bgCtx, cancel := context.WithTimeout(appViewer.NewSystemViewerContext(context.Background()), 30*time.Minute)
		defer cancel()
		if err := s.logSnapshotService.CaptureAll(bgCtx); err != nil {
			s.log.Warnf("scheduled log snapshot: %s", err.Error())
		}
	}()
}

// tickCerts 证书调度探测 + 到期检查(cron 命中或每日兜底)。
func (s *CronScheduler) tickCerts(ctx context.Context) {
	if s.certService == nil {
		return
	}
	if time.Since(s.lastCertCheck) < budgetCheckInterval {
		return
	}
	s.lastCertCheck = time.Now()
	if err := s.certService.RunScheduledProbes(ctx); err != nil {
		s.log.Warnf("cert probe: %s", err.Error())
	}
	if err := s.certService.RunExpiryCheck(ctx); err != nil {
		s.log.Warnf("cert expiry check: %s", err.Error())
	}
}
