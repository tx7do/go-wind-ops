package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/pkg/constants"
)

// BackupService 备份任务管理门面:透传 core 的 BackupService。
type BackupService struct {
	adminV1.BackupServiceHTTPServer

	log *log.Helper

	backupServiceClient opsV1.BackupServiceClient
}

func NewBackupService(ctx *bootstrap.Context, backupServiceClient opsV1.BackupServiceClient) *BackupService {
	return &BackupService{
		log:                 ctx.NewLoggerHelper("backup/service/admin-service"),
		backupServiceClient: backupServiceClient,
	}
}

func (s *BackupService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupTaskResponse, error) {
	return s.backupServiceClient.ListBackupTasks(ctx, req)
}

func (s *BackupService) Get(ctx context.Context, req *opsV1.GetBackupTaskRequest) (*opsV1.BackupTask, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.backupServiceClient.GetBackupTask(ctx, req)
}

func (s *BackupService) Create(ctx context.Context, req *opsV1.CreateBackupTaskRequest) (*opsV1.BackupTask, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.backupServiceClient.CreateBackupTask(ctx, req)
}

func (s *BackupService) Update(ctx context.Context, req *opsV1.UpdateBackupTaskRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.backupServiceClient.UpdateBackupTask(ctx, req)
}

func (s *BackupService) Delete(ctx context.Context, req *opsV1.DeleteBackupTaskRequest) (*emptypb.Empty, error) {
	_, err := s.backupServiceClient.DeleteBackupTask(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *BackupService) Trigger(ctx context.Context, req *opsV1.TriggerBackupRequest) (*opsV1.TriggerBackupResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.backupServiceClient.TriggerBackup(ctx, req)
}

func (s *BackupService) ListRecords(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupRecordResponse, error) {
	return s.backupServiceClient.ListBackupRecords(ctx, req)
}

func (s *BackupService) GetRecordDownloadUrl(ctx context.Context, req *opsV1.GetBackupRecordDownloadUrlRequest) (*opsV1.GetBackupRecordDownloadUrlResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsNodeManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.backupServiceClient.GetBackupRecordDownloadUrl(ctx, req)
}
