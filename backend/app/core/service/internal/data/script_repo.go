package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"
	"go-wind-ops/app/core/service/internal/data/ent/script"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type ScriptRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Script, ent.Script]

	repository *entCrud.Repository[
		ent.ScriptQuery, ent.ScriptSelect,
		ent.ScriptCreate, ent.ScriptCreateBulk,
		ent.ScriptUpdate, ent.ScriptUpdateOne,
		ent.ScriptDelete,
		predicate.Script,
		opsV1.Script, ent.Script,
	]
}

func NewScriptRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ScriptRepo {
	repo := &ScriptRepo{
		log:       ctx.NewLoggerHelper("script/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Script, ent.Script](),
	}

	repo.init()

	return repo
}

func (r *ScriptRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ScriptQuery, ent.ScriptSelect,
		ent.ScriptCreate, ent.ScriptCreateBulk,
		ent.ScriptUpdate, ent.ScriptUpdateOne,
		ent.ScriptDelete,
		predicate.Script,
		opsV1.Script, ent.Script,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *ScriptRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Script.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, opsV1.ErrorInternalServerError("query count failed")
	}

	return count, nil
}

func (r *ScriptRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListScriptResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Script.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListScriptResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListScriptResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ScriptRepo) Get(ctx context.Context, req *opsV1.GetScriptRequest) (*opsV1.Script, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Script.Query()

	dto, err := r.repository.Get(ctx, builder, nil, script.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("script not found")
		}
		r.log.Errorf("query script [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// IsNameTaken 校验脚本名唯一性(excludeID>0 时排除自身,供更新用)。
func (r *ScriptRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Script.Query().Where(script.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(script.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query script name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *ScriptRepo) Create(ctx context.Context, req *opsV1.CreateScriptRequest, operatorID uint32) (*opsV1.Script, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Script.Create().
		SetName(req.Data.GetName()).
		SetNillableDescription(req.Data.Description).
		SetContent(req.Data.GetContent()).
		SetNillableTimeoutSeconds(req.Data.TimeoutSeconds).
		SetLabels(req.Data.Labels)

	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert script failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *ScriptRepo) Update(ctx context.Context, req *opsV1.UpdateScriptRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Script.UpdateOneID(req.GetId())
	builder.Where(script.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Script) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableTimeoutSeconds(req.Data.TimeoutSeconds)

			if req.Data.Content != nil {
				builder.SetContent(req.Data.GetContent())
			}
			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}

			builder.SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(script.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *ScriptRepo) Delete(ctx context.Context, req *opsV1.DeleteScriptRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Script.Delete()
	delBuilder.Where(script.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("script not found")
		}
		r.log.Errorf("delete script [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}
