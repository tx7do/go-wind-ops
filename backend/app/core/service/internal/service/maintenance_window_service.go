// maintenance_window_service.go 告警维护窗口 CRUD。
// 静默语义:窗口激活期间告警行仍落库留痕,仅渠道外呼被跳过
// (suppressible=false 的恢复通知不受影响)——见 SendAlertToChannels。
package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/app/core/service/internal/data"
)

type MaintenanceWindowService struct {
	opsV1.UnimplementedMaintenanceWindowServiceServer

	log  *log.Helper
	repo *data.MaintenanceWindowRepo
}

func NewMaintenanceWindowService(ctx *bootstrap.Context, repo *data.MaintenanceWindowRepo) *MaintenanceWindowService {
	return &MaintenanceWindowService{
		log:  ctx.NewLoggerHelper("maintenance-window/service/core-service"),
		repo: repo,
	}
}

// validateWindow 归一校验:时间对齐 + 结束晚于开始。
func validateWindow(d *opsV1.MaintenanceWindow) (string, time.Time, time.Time, bool, *string, error) {
	if d == nil || d.GetName() == "" || d.GetStartsAt() == nil || d.GetEndsAt() == nil {
		return "", time.Time{}, time.Time{}, false, nil, opsV1.ErrorBadRequest("invalid parameter: name/startsAt/endsAt required")
	}
	starts, ends := d.GetStartsAt().AsTime(), d.GetEndsAt().AsTime()
	if !ends.After(starts) {
		return "", time.Time{}, time.Time{}, false, nil, opsV1.ErrorBadRequest("invalid parameter: endsAt must be after startsAt")
	}
	return d.GetName(), starts, ends, d.GetEnabled(), d.Description, nil
}

func (s *MaintenanceWindowService) ListMaintenanceWindows(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListMaintenanceWindowResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *MaintenanceWindowService) GetMaintenanceWindow(ctx context.Context, req *opsV1.GetMaintenanceWindowRequest) (*opsV1.MaintenanceWindow, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}

func (s *MaintenanceWindowService) CreateMaintenanceWindow(ctx context.Context, req *opsV1.CreateMaintenanceWindowRequest) (*opsV1.MaintenanceWindow, error) {
	name, starts, ends, enabled, desc, err := validateWindow(req.GetData())
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Create(ctx, name, starts, ends, enabled, desc, operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("create maintenance window failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create failed")
	}
	s.log.Infof("maintenance window created (id=%d name=%s)", row.GetId(), name)
	return row, nil
}

func (s *MaintenanceWindowService) UpdateMaintenanceWindow(ctx context.Context, req *opsV1.UpdateMaintenanceWindowRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	name, starts, ends, enabled, desc, err := validateWindow(req.GetData())
	if err != nil {
		return nil, err
	}
	if _, err = s.repo.Update(ctx, req.GetId(), name, starts, ends, enabled, desc); err != nil {
		s.log.Errorf("update maintenance window [%d] failed: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("update failed")
	}
	return &emptypb.Empty{}, nil
}

func (s *MaintenanceWindowService) DeleteMaintenanceWindow(ctx context.Context, req *opsV1.DeleteMaintenanceWindowRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}
