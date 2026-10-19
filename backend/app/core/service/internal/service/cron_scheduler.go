package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/minio/minio-go/v7"
	"github.com/robfig/cron/v3"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/pkg/oss"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// cronSweepInterval 调度扫描周期;cron 表达式最小粒度为分钟,30s 保证
// 不会跨过触发点太久,也不会重复触发(锚点 CAS 去重)。
const cronSweepInterval = 30 * time.Second

// CronScheduler 定时调度器:每 30s 扫描定时构建/脚本/监控/备份任务,命中即触发。
//
// 去重:构建/脚本/备份用 last_cron_at 锚点 + CAS 推进(多副本安全);
// 监控检查只读幂等,重复无害。
type CronScheduler struct {
	ctx           *bootstrap.Context
	buildRepo     *data.BuildRepo
	scriptRepo    *data.ScriptRepo
	monitorRepo   *data.MonitorRepo
	alertRepo     *data.AlertRepo
	projectRepo   *data.ProjectRepo
	nodeRepo      *data.AgentNodeRepo
	backupTaskRepo   *data.BackupTaskRepo
	buildService     *BuildService
	scriptService    *ScriptService
	backupService    *BackupService
	agentService     *AgentService
	notifChannelRepo *data.NotificationChannelRepo
	deliveryRepo     *data.NotificationDeliveryRepo
	budgetRepo       *data.CloudBudgetRepo
	budgetService    *CloudBudgetService
	certRepo         *data.CertRepo
	certService      *CertService
	logSnapshotService *LogSnapshotService
	historyRepo      *data.BuildHistoryRepo
	ossClient        *oss.MinIOClient

	lastRetentionSweep time.Time
	lastBudgetCheck    time.Time
	lastCertCheck      time.Time
	lastLogSnapshot    time.Time
	lastTransientSweep time.Time
	log                *log.Helper
}

func NewCronScheduler(ctx *bootstrap.Context, buildRepo *data.BuildRepo, scriptRepo *data.ScriptRepo, monitorRepo *data.MonitorRepo, alertRepo *data.AlertRepo, projectRepo *data.ProjectRepo, nodeRepo *data.AgentNodeRepo, backupTaskRepo *data.BackupTaskRepo, notifChannelRepo *data.NotificationChannelRepo, deliveryRepo *data.NotificationDeliveryRepo, budgetRepo *data.CloudBudgetRepo, budgetService *CloudBudgetService, certRepo *data.CertRepo, certService *CertService, logSnapshotService *LogSnapshotService, historyRepo *data.BuildHistoryRepo, ossClient *oss.MinIOClient, buildService *BuildService, scriptService *ScriptService, backupService *BackupService, agentService *AgentService) *CronScheduler {
	return &CronScheduler{
		ctx:              ctx,
		buildRepo:        buildRepo,
		scriptRepo:       scriptRepo,
		monitorRepo:      monitorRepo,
		alertRepo:        alertRepo,
		projectRepo:      projectRepo,
		nodeRepo:         nodeRepo,
		backupTaskRepo:   backupTaskRepo,
		notifChannelRepo: notifChannelRepo,
		deliveryRepo:     deliveryRepo,
		budgetRepo:       budgetRepo,
		budgetService:    budgetService,
		certRepo:         certRepo,
		certService:      certService,
		logSnapshotService: logSnapshotService,
		historyRepo:      historyRepo,
		ossClient:        ossClient,
		buildService:     buildService,
		scriptService:    scriptService,
		backupService:    backupService,
		agentService:     agentService,
		log:              ctx.NewLoggerHelper("cron-scheduler/service/core-service"),
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
				s.tickBackups(ctx)
				s.tickBudgets(ctx)
				s.tickCerts(ctx)
				s.tickLogSnapshots(ctx)
				s.tickRetention(ctx)
				s.tickTransientObjects(ctx)
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
		if _, err = s.buildService.TriggerBuild(ctx, &opsV1.TriggerBuildRequest{Id: b.GetId()}); err != nil {
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
		if _, err = s.scriptService.ExecuteScript(ctx, &opsV1.ExecuteScriptRequest{
			Id:     sc.GetId(),
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
		CheckMonitor(ctx, s.log, s.alertRepo, s.notifChannelRepo, s.deliveryRepo, s.projectRepo, s.nodeRepo, s.monitorRepo, s.agentService, m)
	}
}

// cronHitMinute 判断 cron 调度在当前分钟内是否命中。
func cronHitMinute(schedule cron.Schedule, now time.Time) bool {
	next := schedule.Next(now.Add(-time.Minute))
	return !next.After(now)
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
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
