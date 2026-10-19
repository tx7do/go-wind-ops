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

// BuildService 构建配置管理与触发门面:透传 core 的 BuildService。
type BuildService struct {
	adminV1.BuildServiceHTTPServer

	log *log.Helper

	buildServiceClient opsV1.BuildServiceClient
}

func NewBuildService(ctx *bootstrap.Context, buildServiceClient opsV1.BuildServiceClient) *BuildService {
	return &BuildService{
		log:                ctx.NewLoggerHelper("build/service/admin-service"),
		buildServiceClient: buildServiceClient,
	}
}

func (s *BuildService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildResponse, error) {
	return s.buildServiceClient.ListBuilds(ctx, req)
}

func (s *BuildService) Get(ctx context.Context, req *opsV1.GetBuildRequest) (*opsV1.Build, error) {
	return s.buildServiceClient.GetBuild(ctx, req)
}

func (s *BuildService) Create(ctx context.Context, req *opsV1.CreateBuildRequest) (*opsV1.Build, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsBuildManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.buildServiceClient.CreateBuild(ctx, req)
}

func (s *BuildService) Update(ctx context.Context, req *opsV1.UpdateBuildRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsBuildManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.buildServiceClient.UpdateBuild(ctx, req)
}

func (s *BuildService) Delete(ctx context.Context, req *opsV1.DeleteBuildRequest) (*emptypb.Empty, error) {

	// 权限码门(配置面/敏感面):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsBuildManagePermissionCode); err != nil {
		return nil, err
	}
	_, err := s.buildServiceClient.DeleteBuild(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *BuildService) Trigger(ctx context.Context, req *opsV1.TriggerBuildRequest) (*opsV1.TriggerBuildResponse, error) {

	// 权限码门(noop authz 下的真实防线):见 service/permguard.go
	if err := requirePermission(ctx, constants.OpsBuildManagePermissionCode); err != nil {
		return nil, err
	}
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.buildServiceClient.TriggerBuild(ctx, req)
}
