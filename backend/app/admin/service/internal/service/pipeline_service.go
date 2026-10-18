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

// PipelineService 构建流水线门面:透传 core 的配置 CRUD/触发/运行记录。
type PipelineService struct {
	adminV1.PipelineServiceHTTPServer

	log *log.Helper

	pipelineServiceClient opsV1.PipelineServiceClient
}

func NewPipelineService(ctx *bootstrap.Context, pipelineServiceClient opsV1.PipelineServiceClient) *PipelineService {
	return &PipelineService{
		log:                   ctx.NewLoggerHelper("pipeline/service/admin-service"),
		pipelineServiceClient: pipelineServiceClient,
	}
}

func (s *PipelineService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineResponse, error) {
	return s.pipelineServiceClient.ListPipelines(ctx, req)
}

func (s *PipelineService) Get(ctx context.Context, req *opsV1.GetPipelineRequest) (*opsV1.Pipeline, error) {
	return s.pipelineServiceClient.GetPipeline(ctx, req)
}

func (s *PipelineService) Create(ctx context.Context, req *opsV1.CreatePipelineRequest) (*opsV1.Pipeline, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.pipelineServiceClient.CreatePipeline(ctx, req)
}

func (s *PipelineService) Update(ctx context.Context, req *opsV1.UpdatePipelineRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	_, err := s.pipelineServiceClient.UpdatePipeline(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *PipelineService) Delete(ctx context.Context, req *opsV1.DeletePipelineRequest) (*emptypb.Empty, error) {
	_, err := s.pipelineServiceClient.DeletePipeline(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *PipelineService) Trigger(ctx context.Context, req *opsV1.TriggerPipelineRequest) (*opsV1.TriggerPipelineResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.pipelineServiceClient.TriggerPipeline(ctx, req)
}

func (s *PipelineService) ListRuns(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineRunResponse, error) {
	return s.pipelineServiceClient.ListPipelineRuns(ctx, req)
}

func (s *PipelineService) GetRun(ctx context.Context, req *opsV1.GetPipelineRunRequest) (*opsV1.PipelineRun, error) {
	return s.pipelineServiceClient.GetPipelineRun(ctx, req)
}

func (s *PipelineService) GetRunLog(ctx context.Context, req *opsV1.GetPipelineRunLogRequest) (*opsV1.GetPipelineRunLogResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.pipelineServiceClient.GetPipelineRunLog(ctx, req)
}
