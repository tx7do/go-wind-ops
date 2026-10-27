package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/maintenancewindow"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// MaintenanceWindowRepo 告警维护窗口仓储(手写转换,同 api_key 范式)。
type MaintenanceWindowRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewMaintenanceWindowRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *MaintenanceWindowRepo {
	return &MaintenanceWindowRepo{
		log:       ctx.NewLoggerHelper("maintenance-window/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO 实体 → proto。
func (r *MaintenanceWindowRepo) toDTO(e *ent.MaintenanceWindow) *opsV1.MaintenanceWindow {
	return &opsV1.MaintenanceWindow{
		Id:          &e.ID,
		Name:        &e.Name,
		StartsAt:    timeTimestamppb(&e.StartsAt),
		EndsAt:      timeTimestamppb(&e.EndsAt),
		Enabled:     &e.Enabled,
		Description: e.Description,
		CreatedAt:   timeTimestamppb(e.CreatedAt),
	}
}

// List 分页列表。
func (r *MaintenanceWindowRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMaintenanceWindowResponse, error) {
	client := r.entClient.Client().MaintenanceWindow

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(maintenancewindow.FieldID))
	if req != nil && (req.NoPaging == nil || !req.GetNoPaging()) {
		page := req.GetPage()
		if page < 1 {
			page = 1
		}
		pageSize := req.GetPageSize()
		if pageSize < 1 {
			pageSize = 50
		}
		qb = qb.Offset(int((page - 1) * pageSize)).Limit(int(pageSize))
	}

	rows, err := qb.All(ctx)
	if err != nil {
		return nil, err
	}

	resp := &opsV1.ListMaintenanceWindowResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// Get 详情。
func (r *MaintenanceWindowRepo) Get(ctx context.Context, id uint32) (*opsV1.MaintenanceWindow, error) {
	e, err := r.entClient.Client().MaintenanceWindow.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Create 落库。
func (r *MaintenanceWindowRepo) Create(ctx context.Context, name string, startsAt, endsAt time.Time, enabled bool, description *string, operatorID uint32) (*opsV1.MaintenanceWindow, error) {
	builder := r.entClient.Client().MaintenanceWindow.Create().
		SetName(name).
		SetStartsAt(startsAt).
		SetEndsAt(endsAt).
		SetEnabled(enabled)
	if description != nil {
		builder.SetDescription(*description)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Update 名称/时间/启用/描述。
func (r *MaintenanceWindowRepo) Update(ctx context.Context, id uint32, name string, startsAt, endsAt time.Time, enabled bool, description *string) (*opsV1.MaintenanceWindow, error) {
	builder := r.entClient.Client().MaintenanceWindow.UpdateOneID(id).
		SetName(name).
		SetStartsAt(startsAt).
		SetEndsAt(endsAt).
		SetEnabled(enabled)
	if description != nil {
		builder.SetDescription(*description)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Delete 删除。
func (r *MaintenanceWindowRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().MaintenanceWindow.DeleteOneID(id).Exec(ctx)
}

// ActiveWindowName 返回当前激活的维护窗口名(空串=无激活窗口)。
// 任一启用窗口满足 starts_at <= now < ends_at 即激活;命中多个取最先结束的。
func (r *MaintenanceWindowRepo) ActiveWindowName(ctx context.Context, now time.Time) string {
	windows, err := r.entClient.Client().MaintenanceWindow.Query().
		Where(
			maintenancewindow.EnabledEQ(true),
			maintenancewindow.StartsAtLTE(now),
			maintenancewindow.EndsAtGT(now),
		).
		Order(maintenancewindow.ByEndsAt()).
		Limit(1).
		All(ctx)
	if err != nil {
		r.log.Errorf("query active maintenance window: %s", err.Error())
		return ""
	}
	if len(windows) == 0 {
		return ""
	}
	return windows[0].Name
}
