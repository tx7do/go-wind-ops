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
	"go-wind-ops/app/core/service/internal/data/ent/alert"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type AlertRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Alert, ent.Alert]

	repository *entCrud.Repository[
		ent.AlertQuery, ent.AlertSelect,
		ent.AlertCreate, ent.AlertCreateBulk,
		ent.AlertUpdate, ent.AlertUpdateOne,
		ent.AlertDelete,
		predicate.Alert,
		opsV1.Alert, ent.Alert,
	]
}

func NewAlertRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AlertRepo {
	repo := &AlertRepo{
		log:       ctx.NewLoggerHelper("alert/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Alert, ent.Alert](),
	}

	repo.init()

	return repo
}

func (r *AlertRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.AlertQuery, ent.AlertSelect,
		ent.AlertCreate, ent.AlertCreateBulk,
		ent.AlertUpdate, ent.AlertUpdateOne,
		ent.AlertDelete,
		predicate.Alert,
		opsV1.Alert, ent.Alert,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *AlertRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Alert.Query()
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

func (r *AlertRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAlertResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Alert.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListAlertResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListAlertResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *AlertRepo) Get(ctx context.Context, req *opsV1.GetAlertRequest) (*opsV1.Alert, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Alert.Query()

	dto, err := r.repository.Get(ctx, builder, nil, alert.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("alert not found")
		}
		r.log.Errorf("query alert [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// HasUnresolvedAlert 同一监控是否存在未恢复告警(重复告警去重)。
func (r *AlertRepo) HasUnresolvedAlert(ctx context.Context, monitorID uint32) (bool, error) {
	return r.entClient.Client().Alert.
		Query().
		Where(
			alert.MonitorIDEQ(monitorID),
			alert.ResolvedEQ(false),
		).
		Exist(ctx)
}

// Create 创建告警记录(source: MONITOR/BUDGET/CERT)。
func (r *AlertRepo) Create(ctx context.Context, monitorID, projectID, nodeID uint32, source, level, message string, notified bool) (*ent.Alert, error) {
	builder := r.entClient.Client().Alert.
		Create().
		SetMonitorID(monitorID).
		SetProjectID(projectID).
		SetSource(source).
		SetLevel(alert.Level(level)).
		SetMessage(message).
		SetNotified(notified)
	if nodeID > 0 {
		builder.SetNodeID(nodeID)
	}
	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create alert: %s", err.Error())
		return nil, err
	}
	return record, nil
}

// CreateRecovery 创建已恢复的 RECOVERY 告警记录(条件消除通知用,直接终态)。
func (r *AlertRepo) CreateRecovery(ctx context.Context, monitorID, projectID, nodeID uint32, source, message string) (*ent.Alert, error) {
	now := time.Now()
	builder := r.entClient.Client().Alert.
		Create().
		SetMonitorID(monitorID).
		SetProjectID(projectID).
		SetSource(source).
		SetLevel(alert.LevelRecovery).
		SetMessage(message).
		SetResolved(true).
		SetNillableResolvedAt(&now)
	if nodeID > 0 {
		builder.SetNodeID(nodeID)
	}
	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create recovery alert: %s", err.Error())
		return nil, err
	}
	return record, nil
}

// LatestAlertByMonitor 取某监控最近一条告警(重复告警间隔判断用)。
func (r *AlertRepo) LatestAlertByMonitor(ctx context.Context, monitorID uint32) (*ent.Alert, error) {
	return r.entClient.Client().Alert.
		Query().
		Where(alert.MonitorIDEQ(monitorID)).
		Order(ent.Desc(alert.FieldID)).
		First(ctx)
}

// Resolve 标记告警已恢复。
func (r *AlertRepo) Resolve(ctx context.Context, id uint32) error {
	now := time.Now()
	_, err := r.entClient.Client().Alert.
		UpdateOneID(id).
		SetResolved(true).
		SetNillableResolvedAt(&now).
		Save(ctx)
	if ent.IsNotFound(err) {
		return opsV1.ErrorNotFound("alert not found")
	}
	return err
}

// ResolveAllByMonitor 恢复某监控的全部未恢复告警(项目恢复运行时调用)。
func (r *AlertRepo) ResolveAllByMonitor(ctx context.Context, monitorID uint32) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().Alert.
		Update().
		Where(
			alert.MonitorIDEQ(monitorID),
			alert.ResolvedEQ(false),
		).
		SetResolved(true).
		SetNillableResolvedAt(&now).
		Save(ctx)
	return n, err
}

// MarkNotified 标记告警已外呼 webhook。
func (r *AlertRepo) MarkNotified(ctx context.Context, id uint32) error {
	_, err := r.entClient.Client().Alert.
		UpdateOneID(id).
		SetNotified(true).
		Save(ctx)
	return err
}
