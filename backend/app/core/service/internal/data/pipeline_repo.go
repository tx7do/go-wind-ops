package data

import (
	"context"

	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/buildpipeline"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// PipelineRepo 流水线配置仓储:手写转换(steps 为 JSON 文本,
// 与 proto repeated PipelineStep 由服务层互转,不经 copier)。
type PipelineRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewPipelineRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *PipelineRepo {
	return &PipelineRepo{
		log:       ctx.NewLoggerHelper("pipeline/repo/core-service"),
		entClient: entClient,
	}
}

// timeTimestamppb nil/零值时间转 nil。
func timeTimestamppb(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// MarshalPipelineSteps proto 步骤列表 → JSON 文本(protoc 生成结构自带
// 与 json_name 一致的 json tag,encoding/json 直接互转)。
func MarshalPipelineSteps(steps []*opsV1.PipelineStep) (string, error) {
	if len(steps) == 0 {
		return "", nil
	}
	b, err := json.Marshal(steps)
	if err != nil {
		return "", fmt.Errorf("marshal pipeline steps: %w", err)
	}
	return string(b), nil
}

// UnmarshalPipelineSteps JSON 文本 → proto 步骤列表。
func UnmarshalPipelineSteps(text string) ([]*opsV1.PipelineStep, error) {
	if text == "" {
		return nil, nil
	}
	var steps []*opsV1.PipelineStep
	if err := json.Unmarshal([]byte(text), &steps); err != nil {
		return nil, fmt.Errorf("unmarshal pipeline steps: %w", err)
	}
	return steps, nil
}

// MarshalPipelineStepResults 步骤结果列表 → JSON 文本。
func MarshalPipelineStepResults(results []*opsV1.PipelineStepResult) (string, error) {
	if len(results) == 0 {
		return "", nil
	}
	b, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("marshal step results: %w", err)
	}
	return string(b), nil
}

// UnmarshalPipelineStepResults JSON 文本 → 步骤结果列表。
func UnmarshalPipelineStepResults(text string) ([]*opsV1.PipelineStepResult, error) {
	if text == "" {
		return nil, nil
	}
	var results []*opsV1.PipelineStepResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return nil, fmt.Errorf("unmarshal step results: %w", err)
	}
	return results, nil
}

// toDTO ent 实体 → proto(steps JSON 文本转 repeated)。
func (r *PipelineRepo) toDTO(e *ent.BuildPipeline) (*opsV1.Pipeline, error) {
	dto := &opsV1.Pipeline{
		Id:          &e.ID,
		Name:        &e.Name,
		Description: e.Description,
		Labels:      e.Labels,
		CreatedAt:   timeTimestamppb(e.CreatedAt),
		UpdatedAt:   timeTimestamppb(e.UpdatedAt),
	}
	if e.Steps != "" {
		steps, err := UnmarshalPipelineSteps(e.Steps)
		if err != nil {
			return nil, err
		}
		dto.Steps = steps
	}
	return dto, nil
}

// List 分页列表(手写分页,steps 为 JSON 文本不经 copier)。
func (r *PipelineRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListPipelineResponse, error) {
	client := r.entClient.Client().BuildPipeline

	total, err := client.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	qb := client.Query().Order(ent.Desc(buildpipeline.FieldID))
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

	resp := &opsV1.ListPipelineResponse{Total: uint64(total)}
	for _, e := range rows {
		dto, err := r.toDTO(e)
		if err != nil {
			r.log.Warnf("decode pipeline steps (id=%d): %s", e.ID, err.Error())
			continue
		}
		resp.Items = append(resp.Items, dto)
	}
	return resp, nil
}

func (r *PipelineRepo) Get(ctx context.Context, id uint32) (*opsV1.Pipeline, error) {
	e, err := r.entClient.Client().BuildPipeline.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e)
}

// IsNameTaken 校验流水线名唯一性。
func (r *PipelineRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().BuildPipeline.Query().Where(buildpipeline.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(buildpipeline.IDNEQ(excludeID))
	}
	return query.Exist(ctx)
}

// Create 创建(name/steps 必填;steps 由服务层序列化)。
func (r *PipelineRepo) Create(ctx context.Context, name, stepsJSON string, description *string, labels []string, operatorID uint32) (*opsV1.Pipeline, error) {
	builder := r.entClient.Client().BuildPipeline.Create().
		SetName(name).
		SetSteps(stepsJSON).
		SetLabels(labels)
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
	return r.toDTO(e)
}

// Update 更新(stepsJSON 为空串表示不改步骤)。
func (r *PipelineRepo) Update(ctx context.Context, id uint32, name, stepsJSON string, description *string, labels []string) (*opsV1.Pipeline, error) {
	builder := r.entClient.Client().BuildPipeline.UpdateOneID(id)
	if name != "" {
		builder.SetName(name)
	}
	if stepsJSON != "" {
		builder.SetSteps(stepsJSON)
	}
	if description != nil {
		builder.SetDescription(*description)
	}
	if len(labels) > 0 {
		builder.SetLabels(labels)
	}
	e, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toDTO(e)
}

func (r *PipelineRepo) Delete(ctx context.Context, id uint32) error {
	return r.entClient.Client().BuildPipeline.DeleteOneID(id).Exec(ctx)
}
