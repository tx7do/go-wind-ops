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

// DistributionService 构建产物分发门面:透传 core 的 DistributionService。
type DistributionService struct {
	adminV1.DistributionServiceHTTPServer

	log *log.Helper

	distributionServiceClient opsV1.DistributionServiceClient
}

func NewDistributionService(ctx *bootstrap.Context, distributionServiceClient opsV1.DistributionServiceClient) *DistributionService {
	return &DistributionService{
		log:                       ctx.NewLoggerHelper("distribution/service/admin-service"),
		distributionServiceClient: distributionServiceClient,
	}
}

func (s *DistributionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionResponse, error) {
	return s.distributionServiceClient.ListDistributions(ctx, req)
}

func (s *DistributionService) Get(ctx context.Context, req *opsV1.GetDistributionRequest) (*opsV1.Distribution, error) {
	return s.distributionServiceClient.GetDistribution(ctx, req)
}

func (s *DistributionService) Create(ctx context.Context, req *opsV1.CreateDistributionRequest) (*opsV1.Distribution, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}
	return s.distributionServiceClient.CreateDistribution(ctx, req)
}

func (s *DistributionService) Update(ctx context.Context, req *opsV1.UpdateDistributionRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, adminV1.ErrorBadRequest("invalid request")
	}

	req.Data.Id = req.Id

	return s.distributionServiceClient.UpdateDistribution(ctx, req)
}

func (s *DistributionService) Delete(ctx context.Context, req *opsV1.DeleteDistributionRequest) (*emptypb.Empty, error) {
	_, err := s.distributionServiceClient.DeleteDistribution(ctx, req)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *DistributionService) Trigger(ctx context.Context, req *opsV1.TriggerDistributionRequest) (*opsV1.TriggerDistributionResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, adminV1.ErrorBadRequest("invalid request: id required")
	}
	return s.distributionServiceClient.TriggerDistribution(ctx, req)
}

func (s *DistributionService) ListRecords(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListDistributionRecordResponse, error) {
	return s.distributionServiceClient.ListDistributionRecords(ctx, req)
}

func (s *DistributionService) GetRecord(ctx context.Context, req *opsV1.GetDistributionRecordRequest) (*opsV1.DistributionRecord, error) {
	return s.distributionServiceClient.GetDistributionRecord(ctx, req)
}
