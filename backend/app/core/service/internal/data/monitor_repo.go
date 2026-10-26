package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/monitor"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type MonitorRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Monitor, ent.Monitor]

	repository *entCrud.Repository[
		ent.MonitorQuery, ent.MonitorSelect,
		ent.MonitorCreate, ent.MonitorCreateBulk,
		ent.MonitorUpdate, ent.MonitorUpdateOne,
		ent.MonitorDelete,
		predicate.Monitor,
		opsV1.Monitor, ent.Monitor,
	]
}

func NewMonitorRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *MonitorRepo {
	repo := &MonitorRepo{
		log:       ctx.NewLoggerHelper("monitor/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Monitor, ent.Monitor](),
	}

	repo.init()

	return repo
}

func (r *MonitorRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.MonitorQuery, ent.MonitorSelect,
		ent.MonitorCreate, ent.MonitorCreateBulk,
		ent.MonitorUpdate, ent.MonitorUpdateOne,
		ent.MonitorDelete,
		predicate.Monitor,
		opsV1.Monitor, ent.Monitor,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *MonitorRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Monitor.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, opsV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func (r *MonitorRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMonitorResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Monitor.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListMonitorResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListMonitorResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabled 取所有启用的监控规则(调度器用)。
func (r *MonitorRepo) ListEnabled(ctx context.Context) ([]*ent.Monitor, error) {
	return r.entClient.Client().Monitor.
		Query().
		Where(monitor.EnabledEQ(true)).
		All(ctx)
}

func (r *MonitorRepo) Get(ctx context.Context, req *opsV1.GetMonitorRequest) (*opsV1.Monitor, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Monitor.Query()

	dto, err := r.repository.Get(ctx, builder, nil, monitor.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("monitor not found")
		}
		r.log.Errorf("query monitor [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(调度器用)。
func (r *MonitorRepo) GetEntity(ctx context.Context, id uint32) (*ent.Monitor, error) {
	return r.entClient.Client().Monitor.
		Query().
		Where(monitor.IDEQ(id)).
		First(ctx)
}

// IsNameTaken 校验规则名唯一性。
func (r *MonitorRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Monitor.Query().Where(monitor.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(monitor.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query monitor name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *MonitorRepo) Create(ctx context.Context, req *opsV1.CreateMonitorRequest, operatorID uint32) (*opsV1.Monitor, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Monitor.Create().
		SetName(req.Data.GetName()).
		SetProjectID(req.Data.GetProjectId()).
		SetCron(req.Data.GetCron()).
		SetAutoRestart(req.Data.GetAutoRestart()).
		SetEnabled(req.Data.GetEnabled()).
		SetNillableDescription(req.Data.Description)

	// 目标类型分支:项目状态规则(默认)/节点指标规则/进程规则
	targetType := req.Data.GetTargetType()
	if targetType == "" {
		targetType = "PROJECT"
	}
	builder.SetTargetType(monitor.TargetType(targetType))
	builder.SetRepeatIntervalMinutes(int(req.Data.GetRepeatIntervalMinutes()))
	switch targetType {
	case "NODE_METRIC":
		builder.SetNodeID(req.Data.GetNodeId()).
			SetMetricType(monitor.MetricType(req.Data.GetMetricType())).
			SetThresholdPercent(req.Data.GetThresholdPercent()).
			SetConsecutiveCount(int(req.Data.GetConsecutiveCount()))
	case "PROCESS":
		builder.SetNodeID(req.Data.GetNodeId()).
			SetProcessName(req.Data.GetProcessName()).
			SetConsecutiveCount(int(req.Data.GetConsecutiveCount()))
	}
	builder.SetNotifyOnRecovery(req.Data.GetNotifyOnRecovery())

	if req.Data.WebhookUrl != nil {
		builder.SetWebhookURL(req.Data.GetWebhookUrl())
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert monitor failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *MonitorRepo) Update(ctx context.Context, req *opsV1.UpdateMonitorRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Monitor.UpdateOneID(req.GetId())
	builder.Where(monitor.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Monitor) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCron(req.Data.Cron).
				SetNillableWebhookURL(req.Data.WebhookUrl).
				SetNillableDescription(req.Data.Description).
				SetAutoRestart(req.Data.GetAutoRestart()).
				SetEnabled(req.Data.GetEnabled()).
				SetNotifyOnRecovery(req.Data.GetNotifyOnRecovery()).
				SetRepeatIntervalMinutes(int(req.Data.GetRepeatIntervalMinutes()))

			if req.Data.TargetType != nil && req.Data.GetTargetType() != "" {
				builder.SetTargetType(monitor.TargetType(req.Data.GetTargetType()))
			}
			if req.Data.ProjectId != nil && req.Data.GetProjectId() > 0 {
				builder.SetProjectID(req.Data.GetProjectId())
			}
			if req.Data.NodeId != nil && req.Data.GetNodeId() > 0 {
				builder.SetNodeID(req.Data.GetNodeId())
			}
			if req.Data.MetricType != nil && req.Data.GetMetricType() != "" {
				builder.SetMetricType(monitor.MetricType(req.Data.GetMetricType()))
			}
			if req.Data.ProcessName != nil {
				builder.SetProcessName(req.Data.GetProcessName())
			}
			if req.Data.ThresholdPercent != nil {
				builder.SetThresholdPercent(req.Data.GetThresholdPercent())
			}
			if req.Data.ConsecutiveCount != nil && req.Data.GetConsecutiveCount() > 0 {
				builder.SetConsecutiveCount(int(req.Data.GetConsecutiveCount()))
			}
			// 目标类型切换时清掉另一侧的关联,避免残留脏数据
			if req.Data.TargetType != nil {
				switch req.Data.GetTargetType() {
				case "NODE_METRIC":
					builder.SetProjectID(0).SetProcessName("")
				case "PROCESS":
					builder.SetProjectID(0)
				default:
					builder.SetNodeID(0).SetProcessName("")
				}
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(monitor.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *MonitorRepo) Delete(ctx context.Context, req *opsV1.DeleteMonitorRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Monitor.Delete()
	delBuilder.Where(monitor.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("monitor not found")
		}
		r.log.Errorf("delete monitor [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// IncConsecutiveHits 连续命中计数原子自增(SQL 侧 consecutive_hits =
// consecutive_hits + 1),返回自增后的新值供防抖阈值判断与告警文案使用。
// 相比"读出 +1 再写回"的读改写,多副本并发下不会丢失更新。
func (r *MonitorRepo) IncConsecutiveHits(ctx context.Context, id uint32) (int, error) {
	entity, err := r.entClient.Client().Monitor.
		UpdateOneID(id).
		AddConsecutiveHits(1).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return entity.ConsecutiveHits, nil
}

// ResetConsecutiveHits 未命中时计数归零(仅非零行生效,幂等)。
func (r *MonitorRepo) ResetConsecutiveHits(ctx context.Context, id uint32) error {
	_, err := r.entClient.Client().Monitor.
		Update().
		Where(
			monitor.IDEQ(id),
			monitor.ConsecutiveHitsNEQ(0),
		).
		SetConsecutiveHits(0).
		Save(ctx)
	return err
}

// AdvanceCheckAnchorFromNull 给空锚点监控规则落初始锚点(仅 NULL 行生效):
// 新建规则 last_check_at 为 NULL,直接 CAS(EQ)永远匹配不上,导致检查从未
// 触发;首见先落锚不补跑,从下一个周期起正常调度。
func (r *MonitorRepo) AdvanceCheckAnchorFromNull(ctx context.Context, monitorID uint32, now time.Time) (bool, error) {
	n, err := r.entClient.Client().Monitor.
		Update().
		Where(
			monitor.IDEQ(monitorID),
			monitor.LastCheckAtIsNil(),
		).
		SetLastCheckAt(now).
		Save(ctx)
	return n > 0, err
}

// AdvanceCheckAnchor CAS 推进检查锚点:仅当 last_check_at 仍等于 expected
// 时更新为 now,返回是否抢到(多副本去重)——同一规则同一周期只有一方
// 执行检查,探测指令、防抖计数与告警创建随之天然单份。
func (r *MonitorRepo) AdvanceCheckAnchor(ctx context.Context, monitorID uint32, expected, now time.Time) (bool, error) {
	n, err := r.entClient.Client().Monitor.
		Update().
		Where(
			monitor.IDEQ(monitorID),
			monitor.LastCheckAtEQ(expected),
		).
		SetLastCheckAt(now).
		Save(ctx)
	if err != nil {
		r.log.Errorf("advance check anchor [monitor=%d]: %s", monitorID, err.Error())
		return false, err
	}
	return n > 0, nil
}

var _ = time.Now
