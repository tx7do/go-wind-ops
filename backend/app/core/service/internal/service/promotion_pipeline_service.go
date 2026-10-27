// promotion_pipeline_service.go 环境晋升流水线:同一构建产物按有序阶段
// (每阶段=既有分发配置)逐环境晋升。不新造部署路径——Promote = 按指定
// 构建历史触发该阶段分发(缺省该分发配置最新成功构建),产物可追溯复用
// 分发记录;跨阶段节奏由人把握(页面逐阶段晋升)。
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
	"go-wind-ops/app/core/service/internal/data"
)

type PromotionPipelineService struct {
	opsV1.UnimplementedPromotionPipelineServiceServer

	log                 *log.Helper
	repo                *data.PromotionPipelineRepo
	distributionService *DistributionService
}

func NewPromotionPipelineService(ctx *bootstrap.Context, repo *data.PromotionPipelineRepo, distributionService *DistributionService) *PromotionPipelineService {
	return &PromotionPipelineService{
		log:                 ctx.NewLoggerHelper("promotion-pipeline/service/core-service"),
		repo:                repo,
		distributionService: distributionService,
	}
}

// validate 归一校验:名称必填 + 阶段非空。
func (s *PromotionPipelineService) validate(d *opsV1.PromotionPipeline) (string, []*opsV1.PromotionStage, *string, error) {
	if d == nil || d.GetName() == "" {
		return "", nil, nil, opsV1.ErrorBadRequest("invalid parameter: name required")
	}
	if len(d.GetStages()) == 0 {
		return "", nil, nil, opsV1.ErrorBadRequest("invalid parameter: at least one stage required")
	}
	return d.GetName(), d.GetStages(), d.Description, nil
}

func (s *PromotionPipelineService) ListPromotionPipelines(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPromotionPipelineResponse, error) {
	return s.repo.List(ctx, req)
}

func (s *PromotionPipelineService) GetPromotionPipeline(ctx context.Context, req *opsV1.GetPromotionPipelineRequest) (*opsV1.PromotionPipeline, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	return s.repo.Get(ctx, req.GetId())
}

func (s *PromotionPipelineService) CreatePromotionPipeline(ctx context.Context, req *opsV1.CreatePromotionPipelineRequest) (*opsV1.PromotionPipeline, error) {
	name, stages, desc, err := s.validate(req.GetData())
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Create(ctx, name, data.StagesOf(stages), desc, operatorIDOf(ctx))
	if err != nil {
		s.log.Errorf("create promotion pipeline: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create failed")
	}
	return row, nil
}

func (s *PromotionPipelineService) UpdatePromotionPipeline(ctx context.Context, req *opsV1.UpdatePromotionPipelineRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	name, stages, desc, err := s.validate(req.GetData())
	if err != nil {
		return nil, err
	}
	if _, err = s.repo.Update(ctx, req.GetId(), name, data.StagesOf(stages), desc); err != nil {
		s.log.Errorf("update promotion pipeline [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("update failed")
	}
	return &emptypb.Empty{}, nil
}

func (s *PromotionPipelineService) DeletePromotionPipeline(ctx context.Context, req *opsV1.DeletePromotionPipelineRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, opsV1.ErrorInternalServerError("delete failed")
	}
	return &emptypb.Empty{}, nil
}

// PromotePromotion 触发指定阶段的分发:阶段下标越界拒绝;构建历史缺省透传
// (分发触发自带"缺省最新成功构建"语义);结果透传分发记录 ID。
func (s *PromotionPipelineService) PromotePromotion(ctx context.Context, req *opsV1.PromotePromotionRequest) (*opsV1.PromotePromotionResponse, error) {
	if req == nil || req.GetId() == 0 {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}
	e, err := s.repo.GetEntity(ctx, req.GetId())
	if err != nil {
		return nil, opsV1.ErrorNotFound("promotion pipeline not found")
	}
	if int(req.GetStageIndex()) >= len(e.Stages) {
		return nil, opsV1.ErrorBadRequest("invalid parameter: stage_index out of range")
	}
	stage := e.Stages[req.GetStageIndex()]

	resp, err := s.distributionService.TriggerDistribution(ctx, &opsV1.TriggerDistributionRequest{
		Id:        stage.DistributionID,
		HistoryId: req.HistoryId,
	})
	if err != nil {
		s.log.Errorf("promote pipeline [%d] stage [%d] (distribution=%d) failed: %s",
			req.GetId(), req.GetStageIndex(), stage.DistributionID, err.Error())
		return nil, err
	}

	out := &opsV1.PromotePromotionResponse{
		RecordIds: resp.GetRecordIds(),
		Env:       stage.Env,
	}
	s.log.Infof("promotion triggered (pipeline=%d stage=%d env=%s distribution=%d records=%d)",
		req.GetId(), req.GetStageIndex(), stage.Env, stage.DistributionID, len(out.GetRecordIds()))
	return out, nil
}
