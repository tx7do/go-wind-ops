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

// ProjectService 受管机器应用项目管理门面:透传 core 的 ProjectService。
type ProjectService struct {
	adminV1.ProjectServiceHTTPServer

	log *log.Helper

	projectServiceClient opsV1.ProjectServiceClient
}

func NewProjectService(ctx *bootstrap.Context, projectServiceClient opsV1.ProjectServiceClient) *ProjectService {
	return &ProjectService{
		log:                  ctx.NewLoggerHelper("project/service/admin-service"),
		projectServiceClient: projectServiceClient,
	}
}

func (s *ProjectService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListProjectResponse, error) {
	return s.projectServiceClient.ListProjects(ctx, req)
}

func (s *ProjectService) Get(ctx context.Context, req *opsV1.GetProjectRequest) (*opsV1.Project, error) {
	return s.projectServiceClient.GetProject(ctx, req)
}

func (s *ProjectService) Create(ctx context.Context, req *opsV1.CreateProjectRequest) (*opsV1.Project, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.projectServiceClient.CreateProject(ctx, req)
}

func (s *ProjectService) Update(ctx context.Context, req *opsV1.UpdateProjectRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.projectServiceClient.UpdateProject(ctx, req)
}

func (s *ProjectService) Delete(ctx context.Context, req *opsV1.DeleteProjectRequest) (*emptypb.Empty, error) {
	_, err := s.projectServiceClient.DeleteProject(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ProjectService) Operate(ctx context.Context, req *opsV1.OperateProjectRequest) (*opsV1.OperateProjectResponse, error) {
	if req == nil || req.GetOperation() == opsV1.ProjectOperation_PROJECT_OPERATION_UNSPECIFIED {
		return nil, adminV1.ErrorBadRequest("invalid request: operation required")
	}
	return s.projectServiceClient.OperateProject(ctx, req)
}
