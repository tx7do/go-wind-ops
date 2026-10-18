package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// BuildHistoryService 构建历史查询门面:透传 core 的只读查询与产物 URL。
type BuildHistoryService struct {
	adminV1.BuildHistoryServiceHTTPServer

	log *log.Helper

	buildHistoryServiceClient opsV1.BuildHistoryServiceClient
}

func NewBuildHistoryService(ctx *bootstrap.Context, buildHistoryServiceClient opsV1.BuildHistoryServiceClient) *BuildHistoryService {
	return &BuildHistoryService{
		log:                       ctx.NewLoggerHelper("build-history/service/admin-service"),
		buildHistoryServiceClient: buildHistoryServiceClient,
	}
}

func (s *BuildHistoryService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildHistoryResponse, error) {
	return s.buildHistoryServiceClient.ListBuildHistories(ctx, req)
}

func (s *BuildHistoryService) Get(ctx context.Context, req *opsV1.GetBuildHistoryRequest) (*opsV1.BuildHistory, error) {
	return s.buildHistoryServiceClient.GetBuildHistory(ctx, req)
}

func (s *BuildHistoryService) GetArtifactUrl(ctx context.Context, req *opsV1.GetArtifactDownloadUrlRequest) (*opsV1.GetArtifactDownloadUrlResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.buildHistoryServiceClient.GetArtifactDownloadUrl(ctx, req)
}
