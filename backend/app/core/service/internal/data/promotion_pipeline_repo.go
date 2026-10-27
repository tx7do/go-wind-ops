package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/promotionpipeline"
	"go-wind-ops/app/core/service/internal/data/ent/schema"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// PromotionPipelineRepo 环境晋升流水线仓储(手写转换,同 api_key 范式)。
type PromotionPipelineRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewPromotionPipelineRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *PromotionPipelineRepo {
	return &PromotionPipelineRepo{
		log:       ctx.NewLoggerHelper("promotion-pipeline/repo/core-service"),
		entClient: entClient,
	}
}

// toDTO 实体 → proto。
func (r *PromotionPipelineRepo) toDTO(e *ent.PromotionPipeline) *opsV1.PromotionPipeline {
	dto := &opsV1.PromotionPipeline{
		Id:          &e.ID,
		Name:        &e.Name,
		Description: e.Description,
		CreatedAt:   timeTimestamppb(e.CreatedAt),
	}
	for _, st := range e.Stages {
		dto.Stages = append(dto.Stages, &opsV1.PromotionStage{
			DistributionId: st.DistributionID,
			Env:            st.Env,
			Name:           st.Name,
		})
	}
	return dto
}

// List 分页列表。
func (r *PromotionPipelineRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPromotionPipelineResponse, error) {
	client := r.entClient.Client().PromotionPipeline

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(promotionpipeline.FieldID))
	if req != nil && (req.NoPaging == nil || !req.GetNoPaging()) {
		page := req.GetPage()
		if page < 1 {
			page = 1
		}
		pageSize := req.GetPageSize()
		if pageSize < 1 {
			pageSize = 50
		}
		qb = qb.Offset(int((page - 1) * pageSize)).Limit(int(pageSize))
	}

	rows, err := qb.All(ctx)
	if err != nil {
		return nil, err
	}

	resp := &opsV1.ListPromotionPipelineResponse{Total: uint64(total)}
	for _, e := range rows {
		resp.Items = append(resp.Items, r.toDTO(e))
	}
	return resp, nil
}

// Get 详情。
func (r *PromotionPipelineRepo) Get(ctx context.Context, id uint32) (*opsV1.PromotionPipeline, error) {
	e, err := r.entClient.Client().PromotionPipeline.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// GetEntity 实体(Promote 校验阶段下标用)。
func (r *PromotionPipelineRepo) GetEntity(ctx context.Context, id uint32) (*ent.PromotionPipeline, error) {
	return r.entClient.Client().PromotionPipeline.Get(ctx, id)
}

// StagesOf proto 阶段 → 库存结构(服务层 Create/Update 共用)。
func StagesOf(stages []*opsV1.PromotionStage) []schema.PromotionStage {
	out := make([]schema.PromotionStage, 0, len(stages))
	for _, st := range stages {
		if st == nil || st.GetDistributionId() == 0 {
			continue
		}
		out = append(out, schema.PromotionStage{
			DistributionID: st.GetDistributionId(),
			Env:            st.GetEnv(),
			Name:           st.GetName(),
		})
	}
	return out
}

// Create 落库。
func (r *PromotionPipelineRepo) Create(ctx context.Context, name string, stages []schema.PromotionStage, description *string, operatorID uint32) (*opsV1.PromotionPipeline, error) {
	builder := r.entClient.Client().PromotionPipeline.Create().
		SetName(name).
		SetStages(stages)
	if description != nil {
		builder.SetDescription(*description)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Update 名称/阶段/描述。
func (r *PromotionPipelineRepo) Update(ctx context.Context, id uint32, name string, stages []schema.PromotionStage, description *string) (*opsV1.PromotionPipeline, error) {
	builder := r.entClient.Client().PromotionPipeline.UpdateOneID(id).
		SetName(name).
		SetStages(stages)
	if description != nil {
		builder.SetDescription(*description)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e), nil
}

// Delete 删除。
func (r *PromotionPipelineRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().PromotionPipeline.DeleteOneID(id).Exec(ctx)
}
