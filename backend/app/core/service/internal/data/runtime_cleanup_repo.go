// runtime_cleanup_repo.go 运行数据保留期清扫:告警/通知投递台账/节点指标
// 三张高频写入表在无限累积下拖慢查询与膨胀存储,按天数保留(0=不清理)。
// 与审计留存(audit_retention_repo,AUDIT_RETENTION_DAYS)分立:审计是合规
// 数据默认 90 天,运行数据默认更短。
package data

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
)

// RuntimeCleanupRepo 运行数据保留期清扫仓储。
type RuntimeCleanupRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewRuntimeCleanupRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *RuntimeCleanupRepo {
	return &RuntimeCleanupRepo{
		log:       ctx.NewLoggerHelper("runtime-cleanup/repo/core-service"),
		entClient: entClient,
	}
}

// retentionEnv 环境变量名 → 缺省天数。
type retentionEnv struct {
	env       string
	days      int
	table     string
	timeCol   string
	itemLabel string
}

var runtimeRetentionRules = []retentionEnv{
	{"ALERT_RETENTION_DAYS", 90, "ops_alerts", "created_at", "告警"},
	{"DELIVERY_RETENTION_DAYS", 90, "ops_notification_deliveries", "created_at", "通知投递"},
	{"NODE_METRIC_RETENTION_DAYS", 30, "ops_node_metrics", "created_at", "节点指标"},
}

func retentionDays(env string, def int) (int, bool) {
	v := strings.TrimSpace(os.Getenv(env))
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def, true
	}
	return n, n > 0
}

// SweepRuntimeRetention 清扫过期运行数据,返回逐项删除行数。
func (r *RuntimeCleanupRepo) SweepRuntimeRetention(ctx context.Context, now time.Time) map[string]int64 {
	out := map[string]int64{}
	db := r.entClient.DB()
	for _, rule := range runtimeRetentionRules {
		days, enabled := retentionDays(rule.env, rule.days)
		if !enabled {
			continue
		}
		cutoff := now.AddDate(0, 0, -days)
		res, err := db.ExecContext(ctx,
			"DELETE FROM "+rule.table+" WHERE "+rule.timeCol+" < $1", cutoff)
		if err != nil {
			r.log.Errorf("retention sweep %s failed: %s", rule.table, err.Error())
			out[rule.itemLabel] = -1
			continue
		}
		n, _ := res.RowsAffected()
		out[rule.itemLabel] = n
	}
	return out
}
