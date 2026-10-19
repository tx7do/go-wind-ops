package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"
	"go-wind-ops/app/core/service/internal/data/ent/scriptbatch"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type ScriptBatchRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.ScriptBatch, ent.ScriptBatch]

	repository *entCrud.Repository[
		ent.ScriptBatchQuery, ent.ScriptBatchSelect,
		ent.ScriptBatchCreate, ent.ScriptBatchCreateBulk,
		ent.ScriptBatchUpdate, ent.ScriptBatchUpdateOne,
		ent.ScriptBatchDelete,
		predicate.ScriptBatch,
		opsV1.ScriptBatch, ent.ScriptBatch,
	]
}

func NewScriptBatchRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ScriptBatchRepo {
	repo := &ScriptBatchRepo{
		log:       ctx.NewLoggerHelper("script-batch/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.ScriptBatch, ent.ScriptBatch](),
	}

	repo.init()

	return repo
}

func (r *ScriptBatchRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ScriptBatchQuery, ent.ScriptBatchSelect,
		ent.ScriptBatchCreate, ent.ScriptBatchCreateBulk,
		ent.ScriptBatchUpdate, ent.ScriptBatchUpdateOne,
		ent.ScriptBatchDelete,
		predicate.ScriptBatch,
		opsV1.ScriptBatch, ent.ScriptBatch,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *ScriptBatchRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptBatchResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ScriptBatch.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListScriptBatchResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListScriptBatchResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ScriptBatchRepo) Get(ctx context.Context, id uint32) (*opsV1.ScriptBatch, error) {
	builder := r.entClient.Client().ScriptBatch.Query()

	dto, err := r.repository.Get(ctx, builder, nil, scriptbatch.IDEQ(id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("script batch not found")
		}
		r.log.Errorf("query script batch [%d]: %s", id, err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(批次计数对账用)。
func (r *ScriptBatchRepo) GetEntity(ctx context.Context, id uint32) (*ent.ScriptBatch, error) {
	return r.entClient.Client().ScriptBatch.
		Query().
		Where(scriptbatch.IDEQ(id)).
		First(ctx)
}

// Create 创建批次(RUNNING 起步,total 为目标节点数)。
func (r *ScriptBatchRepo) Create(ctx context.Context, scriptID uint32, scriptName string, nodeIDs []uint32, total uint32, operatorID uint32) (*ent.ScriptBatch, error) {
	builder := r.entClient.Client().ScriptBatch.
		Create().
		SetScriptID(scriptID).
		SetScriptName(scriptName).
		SetNodeIds(nodeIDs).
		SetTotal(total).
		SetStatus(scriptbatch.StatusRunning)
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create script batch: %s", err.Error())
		return nil, err
	}
	return record, nil
}

// ApplyItemResult 批次内单节点到达终态:原子累加成败计数,全部终态时
// 把批次推进到 FINISHED。幂等依赖调用方只在台账首次定案时调用。
func (r *ScriptBatchRepo) ApplyItemResult(ctx context.Context, batchID uint32, success bool) error {
	builder := r.entClient.Client().ScriptBatch.
		Update().
		Where(scriptbatch.IDEQ(batchID))
	if success {
		builder.AddSuccessCount(1)
	} else {
		builder.AddFailedCount(1)
	}
	if _, err := builder.Save(ctx); err != nil {
		return err
	}

	b, err := r.GetEntity(ctx, batchID)
	if err != nil {
		return err
	}
	if b.Status == scriptbatch.StatusRunning && b.SuccessCount+b.FailedCount >= b.Total {
		_, err = r.entClient.Client().ScriptBatch.
			UpdateOneID(batchID).
			SetStatus(scriptbatch.StatusFinished).
			Save(ctx)
	}
	return err
}
