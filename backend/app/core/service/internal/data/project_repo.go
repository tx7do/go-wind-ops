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
	"go-wind-ops/app/core/service/internal/data/ent/project"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type ProjectRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper        *mapper.CopierMapper[opsV1.Project, ent.Project]
	typeConverter *mapper.EnumTypeConverter[opsV1.ProjectStatus, project.LastStatus]

	repository *entCrud.Repository[
		ent.ProjectQuery, ent.ProjectSelect,
		ent.ProjectCreate, ent.ProjectCreateBulk,
		ent.ProjectUpdate, ent.ProjectUpdateOne,
		ent.ProjectDelete,
		predicate.Project,
		opsV1.Project, ent.Project,
	]
}

func NewProjectRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ProjectRepo {
	repo := &ProjectRepo{
		log:           ctx.NewLoggerHelper("project/repo/core-service"),
		entClient:     entClient,
		mapper:        mapper.NewCopierMapper[opsV1.Project, ent.Project](),
		typeConverter: mapper.NewEnumTypeConverter[opsV1.ProjectStatus, project.LastStatus](opsV1.ProjectStatus_name, opsV1.ProjectStatus_value),
	}

	repo.init()

	return repo
}

func (r *ProjectRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ProjectQuery, ent.ProjectSelect,
		ent.ProjectCreate, ent.ProjectCreateBulk,
		ent.ProjectUpdate, ent.ProjectUpdateOne,
		ent.ProjectDelete,
		predicate.Project,
		opsV1.Project, ent.Project,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *ProjectRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Project.Query()
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

func (r *ProjectRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListProjectResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Project.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListProjectResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListProjectResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ProjectRepo) Get(ctx context.Context, req *opsV1.GetProjectRequest) (*opsV1.Project, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Project.Query()

	dto, err := r.repository.Get(ctx, builder, nil, project.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("project not found")
		}
		r.log.Errorf("query project [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// IsNameTaken 校验节点内项目名唯一性。
func (r *ProjectRepo) IsNameTaken(ctx context.Context, nodeID uint32, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Project.Query().
		Where(
			project.NodeIDEQ(nodeID),
			project.NameEQ(name),
		)
	if excludeID > 0 {
		query = query.Where(project.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query project name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *ProjectRepo) Create(ctx context.Context, req *opsV1.CreateProjectRequest, operatorID uint32) (*opsV1.Project, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Project.Create().
		SetNodeID(req.Data.GetNodeId()).
		SetName(req.Data.GetName()).
		SetNillableDescription(req.Data.Description).
		SetProjectDir(req.Data.GetProjectDir()).
		SetNillableStartCommand(req.Data.StartCommand).
		SetNillableStopCommand(req.Data.StopCommand).
		SetNillableStatusCommand(req.Data.StatusCommand).
		SetNillableEnv(req.Data.Env).
		SetLabels(req.Data.Labels)

	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert project failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *ProjectRepo) Update(ctx context.Context, req *opsV1.UpdateProjectRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Project.UpdateOneID(req.GetId())
	builder.Where(project.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Project) {
			builder.
				SetNillableNodeID(req.Data.NodeId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableProjectDir(req.Data.ProjectDir).
				SetNillableStartCommand(req.Data.StartCommand).
				SetNillableStopCommand(req.Data.StopCommand).
				SetNillableStatusCommand(req.Data.StatusCommand).
				SetNillableEnv(req.Data.Env)

			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}

			builder.SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(project.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *ProjectRepo) Delete(ctx context.Context, req *opsV1.DeleteProjectRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Project.Delete()
	delBuilder.Where(project.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("project not found")
		}
		r.log.Errorf("delete project [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// UpdateRuntime 回填 agent 上报的项目运行时状态(last_pid/last_status)。
// 项目不存在时静默跳过(可能已被删除)。
func (r *ProjectRepo) UpdateRuntime(ctx context.Context, projectID uint32, pid uint64, status string) error {
	now := time.Now()
	builder := r.entClient.Client().Project.
		UpdateOneID(projectID).
		SetNillableLastPid(&pid).
		SetLastStatus(project.LastStatus(status)).
		SetNillableLastStatusAt(&now)
	if _, err := builder.Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
