package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// RepositoryService 构建源仓库管理门面:透传 core 的 RepositoryService。
type RepositoryService struct {
	adminV1.RepositoryServiceHTTPServer

	log *log.Helper

	repositoryServiceClient opsV1.RepositoryServiceClient
}

func NewRepositoryService(ctx *bootstrap.Context, repositoryServiceClient opsV1.RepositoryServiceClient) *RepositoryService {
	return &RepositoryService{
		log:                     ctx.NewLoggerHelper("repository/service/admin-service"),
		repositoryServiceClient: repositoryServiceClient,
	}
}

func (s *RepositoryService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListRepositoryResponse, error) {
	return s.repositoryServiceClient.ListRepositories(ctx, req)
}

func (s *RepositoryService) Get(ctx context.Context, req *opsV1.GetRepositoryRequest) (*opsV1.Repository, error) {
	return s.repositoryServiceClient.GetRepository(ctx, req)
}

func (s *RepositoryService) Create(ctx context.Context, req *opsV1.CreateRepositoryRequest) (*opsV1.Repository, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.repositoryServiceClient.CreateRepository(ctx, req)
}

func (s *RepositoryService) Update(ctx context.Context, req *opsV1.UpdateRepositoryRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.repositoryServiceClient.UpdateRepository(ctx, req)
}

func (s *RepositoryService) Delete(ctx context.Context, req *opsV1.DeleteRepositoryRequest) (*emptypb.Empty, error) {
	_, err := s.repositoryServiceClient.DeleteRepository(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
