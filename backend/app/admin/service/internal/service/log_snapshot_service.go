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

// LogSnapshotService 日志快照门面:透传 core。
type LogSnapshotService struct {
	adminV1.LogSnapshotServiceHTTPServer

	log *log.Helper

	logSnapshotServiceClient opsV1.LogSnapshotServiceClient
}

func NewLogSnapshotService(ctx *bootstrap.Context, logSnapshotServiceClient opsV1.LogSnapshotServiceClient) *LogSnapshotService {
	return &LogSnapshotService{
		log:                      ctx.NewLoggerHelper("log-snapshot/service/admin-service"),
		logSnapshotServiceClient: logSnapshotServiceClient,
	}
}

func (s *LogSnapshotService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListLogSnapshotResponse, error) {
	return s.logSnapshotServiceClient.ListLogSnapshots(ctx, req)
}

func (s *LogSnapshotService) Get(ctx context.Context, req *opsV1.GetLogSnapshotRequest) (*opsV1.LogSnapshot, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.logSnapshotServiceClient.GetLogSnapshot(ctx, req)
}

func (s *LogSnapshotService) Capture(ctx context.Context, req *opsV1.CaptureLogSnapshotRequest) (*opsV1.LogSnapshot, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetProjectId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: projectId required")
	}
	return s.logSnapshotServiceClient.CaptureLogSnapshot(ctx, req)
}

func (s *LogSnapshotService) Search(ctx context.Context, req *opsV1.SearchLogSnapshotsRequest) (*opsV1.SearchLogSnapshotResponse, error) {
	if req == nil || req.GetKeyword() == "" {
		return nil, adminV1.ErrorBadRequest("invalid request: keyword required")
	}
	return s.logSnapshotServiceClient.SearchLogSnapshots(ctx, req)
}

func (s *LogSnapshotService) Delete(ctx context.Context, req *opsV1.DeleteLogSnapshotRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsProjectManagePermissionCode); err != nil {
		return nil, err
	}
	_, err := s.logSnapshotServiceClient.DeleteLogSnapshot(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
