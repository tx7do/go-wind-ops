// audit_retention_repo.go 五类审计日志的留存清理仓储。
// 审计表只写不删、无界增长(此前仅终端回放有留存清理)——按创建时间批量
// 删除早于留存期的行,单表单批限量防长事务。
package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/apiauditlog"
	"go-wind-ops/app/core/service/internal/data/ent/dataaccessauditlog"
	"go-wind-ops/app/core/service/internal/data/ent/loginauditlog"
	"go-wind-ops/app/core/service/internal/data/ent/operationauditlog"
	"go-wind-ops/app/core/service/internal/data/ent/permissionauditlog"
)

// VerifyChains 审计域完整性核验(cron 自检用,与 tools/audit-verify 同语义:
// 只判哈希链,ECDSA 验签仍在离线 CLI)。
func (r *AuditRetentionRepo) VerifyChains(ctx context.Context) ([]ChainIssue, error) {
	return VerifyAuditChains(ctx, r.entClient.DB())
}

// AuditRetentionRepo 审计日志留存清理。
type AuditRetentionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewAuditRetentionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AuditRetentionRepo {
	return &AuditRetentionRepo{
		entClient: entClient,
		log:       ctx.NewLoggerHelper("audit-retention/data/core-service"),
	}
}

// DeleteOlderThan 删除早于 cutoff 的审计行(操作/登录/API/数据访问/权限),
// 单表单批不超过 limit(ent 删除构造器不支持 Limit,先取 ID 批再按 ID 删),
// 返回删除总行数。
func (r *AuditRetentionRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	client := r.entClient.Client()
	var total int64

	sweep := func(table string, queryIDs func(context.Context) ([]uint32, error), deleteIDs func(context.Context, []uint32) (int, error)) {
		ids, err := queryIDs(ctx)
		if err != nil {
			r.log.Warnf("audit retention sweep %s select: %s", table, err.Error())
			return
		}
		if len(ids) == 0 {
			return
		}
		n, err := deleteIDs(ctx, ids)
		if err != nil {
			r.log.Warnf("audit retention sweep %s delete: %s", table, err.Error())
			return
		}
		total += int64(n)
	}

	sweep("operation",
		func(ctx context.Context) ([]uint32, error) {
			return client.OperationAuditLog.Query().
				Where(operationauditlog.CreatedAtLT(cutoff)).
				Limit(limit).IDs(ctx)
		},
		func(ctx context.Context, ids []uint32) (int, error) {
			return client.OperationAuditLog.Delete().Where(operationauditlog.IDIn(ids...)).Exec(ctx)
		})
	sweep("login",
		func(ctx context.Context) ([]uint32, error) {
			return client.LoginAuditLog.Query().
				Where(loginauditlog.CreatedAtLT(cutoff)).
				Limit(limit).IDs(ctx)
		},
		func(ctx context.Context, ids []uint32) (int, error) {
			return client.LoginAuditLog.Delete().Where(loginauditlog.IDIn(ids...)).Exec(ctx)
		})
	sweep("api",
		func(ctx context.Context) ([]uint32, error) {
			return client.ApiAuditLog.Query().
				Where(apiauditlog.CreatedAtLT(cutoff)).
				Limit(limit).IDs(ctx)
		},
		func(ctx context.Context, ids []uint32) (int, error) {
			return client.ApiAuditLog.Delete().Where(apiauditlog.IDIn(ids...)).Exec(ctx)
		})
	sweep("data-access",
		func(ctx context.Context) ([]uint32, error) {
			return client.DataAccessAuditLog.Query().
				Where(dataaccessauditlog.CreatedAtLT(cutoff)).
				Limit(limit).IDs(ctx)
		},
		func(ctx context.Context, ids []uint32) (int, error) {
			return client.DataAccessAuditLog.Delete().Where(dataaccessauditlog.IDIn(ids...)).Exec(ctx)
		})
	sweep("permission",
		func(ctx context.Context) ([]uint32, error) {
			return client.PermissionAuditLog.Query().
				Where(permissionauditlog.CreatedAtLT(cutoff)).
				Limit(limit).IDs(ctx)
		},
		func(ctx context.Context, ids []uint32) (int, error) {
			return client.PermissionAuditLog.Delete().Where(permissionauditlog.IDIn(ids...)).Exec(ctx)
		})

	return total, nil
}
