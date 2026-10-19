package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// CloudCostService 云成本月账单门面:透传 core。
type CloudCostService struct {
	adminV1.CloudCostServiceHTTPServer

	log *log.Helper

	cloudCostServiceClient opsV1.CloudCostServiceClient
}

func NewCloudCostService(ctx *bootstrap.Context, cloudCostServiceClient opsV1.CloudCostServiceClient) *CloudCostService {
	return &CloudCostService{
		log:                    ctx.NewLoggerHelper("cloud-cost/service/admin-service"),
		cloudCostServiceClient: cloudCostServiceClient,
	}
}

func (s *CloudCostService) List(ctx context.Context, req *opsV1.ListCloudCostsRequest) (*opsV1.ListCloudCostsResponse, error) {
	if req == nil {
		req = &opsV1.ListCloudCostsRequest{}
	}
	return s.cloudCostServiceClient.ListCloudCosts(ctx, req)
}

func (s *CloudCostService) Upsert(ctx context.Context, req *opsV1.UpsertCloudCostRequest) (*opsV1.CloudCost, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.cloudCostServiceClient.UpsertCloudCost(ctx, req)
}

func (s *CloudCostService) Delete(ctx context.Context, req *opsV1.DeleteCloudCostRequest) (*emptypb.Empty, error) {
	_, err := s.cloudCostServiceClient.DeleteCloudCost(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CloudCostService) Import(ctx context.Context, req *opsV1.ImportCloudCostsRequest) (*opsV1.ImportCloudCostsResponse, error) {
	if req == nil || len(req.GetRows()) == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: rows required")
	}
	return s.cloudCostServiceClient.ImportCloudCosts(ctx, req)
}

func (s *CloudCostService) Summary(ctx context.Context, req *opsV1.GetCloudCostSummaryRequest) (*opsV1.GetCloudCostSummaryResponse, error) {
	if req == nil {
		req = &opsV1.GetCloudCostSummaryRequest{}
	}
	return s.cloudCostServiceClient.GetCloudCostSummary(ctx, req)
}
