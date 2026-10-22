package data

import (
	"time"
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/build"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type BuildRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Build, ent.Build]

	repository *entCrud.Repository[
		ent.BuildQuery, ent.BuildSelect,
		ent.BuildCreate, ent.BuildCreateBulk,
		ent.BuildUpdate, ent.BuildUpdateOne,
		ent.BuildDelete,
		predicate.Build,
		opsV1.Build, ent.Build,
	]
}

func NewBuildRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BuildRepo {
	repo := &BuildRepo{
		log:       ctx.NewLoggerHelper("build/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Build, ent.Build](),
	}

	repo.init()

	return repo
}

func (r *BuildRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.BuildQuery, ent.BuildSelect,
		ent.BuildCreate, ent.BuildCreateBulk,
		ent.BuildUpdate, ent.BuildUpdateOne,
		ent.BuildDelete,
		predicate.Build,
		opsV1.Build, ent.Build,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *BuildRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Build.Query()
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

func (r *BuildRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBuildResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Build.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListBuildResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListBuildResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *BuildRepo) Get(ctx context.Context, req *opsV1.GetBuildRequest) (*opsV1.Build, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Build.Query()

	dto, err := r.repository.Get(ctx, builder, nil, build.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("build not found")
		}
		r.log.Errorf("query build [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// IsNameTaken 校验构建名唯一性。
func (r *BuildRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().Build.Query().Where(build.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(build.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query build name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *BuildRepo) Create(ctx context.Context, req *opsV1.CreateBuildRequest, operatorID uint32) (*opsV1.Build, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Build.Create().
		SetName(req.Data.GetName()).
		SetRepositoryID(req.Data.GetRepositoryId()).
		SetBuildCommand(req.Data.GetBuildCommand()).
		SetArtifactPath(req.Data.GetArtifactPath()).
		SetNillableDescription(req.Data.Description).
		SetLabels(req.Data.Labels).
		SetNillableCron(req.Data.Cron)

	if req.Data.Branch != nil {
		builder.SetBranch(req.Data.GetBranch())
	}
	if req.Data.TimeoutSeconds != nil {
		builder.SetTimeoutSeconds(req.Data.GetTimeoutSeconds()).
			SetNillableArtifactRetention(req.Data.ArtifactRetention)
	}
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert build failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *BuildRepo) Update(ctx context.Context, req *opsV1.UpdateBuildRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Build.UpdateOneID(req.GetId())
	builder.Where(build.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.Build) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableBranch(req.Data.Branch).
				SetNillableBuildCommand(req.Data.BuildCommand).
				SetNillableArtifactPath(req.Data.ArtifactPath).
				SetNillableTimeoutSeconds(req.Data.TimeoutSeconds).
				SetNillableArtifactRetention(req.Data.ArtifactRetention).
				SetNillableDescription(req.Data.Description).
				SetNillableCron(req.Data.Cron)

			if req.Data.RepositoryId != nil && req.Data.GetRepositoryId() > 0 {
				builder.SetRepositoryID(req.Data.GetRepositoryId())
			}
			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(build.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *BuildRepo) Delete(ctx context.Context, req *opsV1.DeleteBuildRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().Build.Delete()
	delBuilder.Where(build.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("build not found")
		}
		r.log.Errorf("delete build [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// AdvanceCronAnchorFromNull 给空锚点任务落初始锚点(仅 NULL 行生效):
// 新建任务 last_cron_at 为 NULL,直接 CAS(EQ)永远匹配不上,导致定时
// 调度从未触发;首见先落锚不补跑,从下一个周期起正常调度。
func (r *BuildRepo) AdvanceCronAnchorFromNull(ctx context.Context, buildID uint32, now time.Time) (bool, error) {
	n, err := r.entClient.Client().Build.
		Update().
		Where(
			build.IDEQ(buildID),
			build.LastCronAtIsNil(),
		).
		SetLastCronAt(now).
		Save(ctx)
	return n > 0, err
}

// AdvanceCronAnchor CAS 推进定时调度锚点:仅当 last_cron_at 仍等于 expected
// 时更新为 now,返回是否抢到(多副本去重)。
func (r *BuildRepo) AdvanceCronAnchor(ctx context.Context, buildID uint32, expected, now time.Time) (bool, error) {
	n, err := r.entClient.Client().Build.
		Update().
		Where(
			build.IDEQ(buildID),
			build.LastCronAtEQ(expected),
		).
		SetLastCronAt(now).
		Save(ctx)
	if err != nil {
		r.log.Errorf("advance cron anchor [build=%d]: %s", buildID, err.Error())
		return false, err
	}
	return n > 0, nil
}
