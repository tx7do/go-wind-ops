// promotion_pipeline_service.go 环境晋升流水线 BFF 门面:CRUD+晋升透传 core。
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	adminV1 "go-wind-ops/api/gen/go/admin/service/v1"
	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type PromotionPipelineService struct {
	adminV1.PromotionPipelineServiceHTTPServer

	log                            *log.Helper
	promotionPipelineServiceClient opsV1.PromotionPipelineServiceClient
}

func NewPromotionPipelineService(
	ctx *bootstrap.Context,
	promotionPipelineServiceClient opsV1.PromotionPipelineServiceClient,
) *PromotionPipelineService {
	return &PromotionPipelineService{
		log:                            ctx.NewLoggerHelper("promotion-pipeline/service/admin-service"),
		promotionPipelineServiceClient: promotionPipelineServiceClient,
	}
}

func (s *PromotionPipelineService) ListPromotionPipelines(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPromotionPipelineResponse, error) {
	return s.promotionPipelineServiceClient.ListPromotionPipelines(ctx, req)
}

func (s *PromotionPipelineService) GetPromotionPipeline(ctx context.Context, req *opsV1.GetPromotionPipelineRequest) (*opsV1.PromotionPipeline, error) {
	return s.promotionPipelineServiceClient.GetPromotionPipeline(ctx, req)
}

func (s *PromotionPipelineService) CreatePromotionPipeline(ctx context.Context, req *opsV1.CreatePromotionPipelineRequest) (*opsV1.PromotionPipeline, error) {
	return s.promotionPipelineServiceClient.CreatePromotionPipeline(ctx, req)
}

func (s *PromotionPipelineService) UpdatePromotionPipeline(ctx context.Context, req *opsV1.UpdatePromotionPipelineRequest) (*emptypb.Empty, error) {
	return s.promotionPipelineServiceClient.UpdatePromotionPipeline(ctx, req)
}

func (s *PromotionPipelineService) DeletePromotionPipeline(ctx context.Context, req *opsV1.DeletePromotionPipelineRequest) (*emptypb.Empty, error) {
	return s.promotionPipelineServiceClient.DeletePromotionPipeline(ctx, req)
}

// PromotePromotion 晋升(触发指定阶段分发)。
func (s *PromotionPipelineService) PromotePromotion(ctx context.Context, req *opsV1.PromotePromotionRequest) (*opsV1.PromotePromotionResponse, error) {
	return s.promotionPipelineServiceClient.PromotePromotion(ctx, req)
}
