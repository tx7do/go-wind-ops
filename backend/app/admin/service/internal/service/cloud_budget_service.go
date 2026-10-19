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

// CloudBudgetService 云成本预算门面:透传 core 的 CloudBudgetService。
type CloudBudgetService struct {
	adminV1.CloudBudgetServiceHTTPServer

	log *log.Helper

	cloudBudgetServiceClient opsV1.CloudBudgetServiceClient
}

func NewCloudBudgetService(ctx *bootstrap.Context, cloudBudgetServiceClient opsV1.CloudBudgetServiceClient) *CloudBudgetService {
	return &CloudBudgetService{
		log:                      ctx.NewLoggerHelper("cloud-budget/service/admin-service"),
		cloudBudgetServiceClient: cloudBudgetServiceClient,
	}
}

func (s *CloudBudgetService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListCloudBudgetResponse, error) {
	return s.cloudBudgetServiceClient.ListCloudBudgets(ctx, req)
}

func (s *CloudBudgetService) Get(ctx context.Context, req *opsV1.GetCloudBudgetRequest) (*opsV1.CloudBudget, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.cloudBudgetServiceClient.GetCloudBudget(ctx, req)
}

func (s *CloudBudgetService) Create(ctx context.Context, req *opsV1.CreateCloudBudgetRequest) (*opsV1.CloudBudget, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.cloudBudgetServiceClient.CreateCloudBudget(ctx, req)
}

func (s *CloudBudgetService) Update(ctx context.Context, req *opsV1.UpdateCloudBudgetRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}


	return s.cloudBudgetServiceClient.UpdateCloudBudget(ctx, req)
}

func (s *CloudBudgetService) Delete(ctx context.Context, req *opsV1.DeleteCloudBudgetRequest) (*emptypb.Empty, error) {
	_, err := s.cloudBudgetServiceClient.DeleteCloudBudget(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudBudgetService) Check(ctx context.Context, req *opsV1.CheckCloudBudgetRequest) (*emptypb.Empty, error) {
	return s.cloudBudgetServiceClient.CheckCloudBudget(ctx, req)
}
