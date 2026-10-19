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
	"go-wind-ops/app/core/service/internal/data/ent/backuptask"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type BackupTaskRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.BackupTask, ent.BackupTask]

	repository *entCrud.Repository[
		ent.BackupTaskQuery, ent.BackupTaskSelect,
		ent.BackupTaskCreate, ent.BackupTaskCreateBulk,
		ent.BackupTaskUpdate, ent.BackupTaskUpdateOne,
		ent.BackupTaskDelete,
		predicate.BackupTask,
		opsV1.BackupTask, ent.BackupTask,
	]
}

func NewBackupTaskRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BackupTaskRepo {
	repo := &BackupTaskRepo{
		log:       ctx.NewLoggerHelper("backup-task/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.BackupTask, ent.BackupTask](),
	}

	repo.init()

	return repo
}

func (r *BackupTaskRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.BackupTaskQuery, ent.BackupTaskSelect,
		ent.BackupTaskCreate, ent.BackupTaskCreateBulk,
		ent.BackupTaskUpdate, ent.BackupTaskUpdateOne,
		ent.BackupTaskDelete,
		predicate.BackupTask,
		opsV1.BackupTask, ent.BackupTask,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *BackupTaskRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupTaskResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupTask.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListBackupTaskResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListBackupTaskResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabledWithCron 取所有启用且配置了 cron 的任务(调度器用)。
func (r *BackupTaskRepo) ListEnabledWithCron(ctx context.Context) ([]*ent.BackupTask, error) {
	return r.entClient.Client().BackupTask.
		Query().
		Where(
			backuptask.EnabledEQ(true),
			backuptask.CronNEQ(""),
		).
		All(ctx)
}

func (r *BackupTaskRepo) Get(ctx context.Context, req *opsV1.GetBackupTaskRequest) (*opsV1.BackupTask, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupTask.Query()

	dto, err := r.repository.Get(ctx, builder, nil, backuptask.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("backup task not found")
		}
		r.log.Errorf("query backup task [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(触发备份用)。
func (r *BackupTaskRepo) GetEntity(ctx context.Context, id uint32) (*ent.BackupTask, error) {
	return r.entClient.Client().BackupTask.
		Query().
		Where(backuptask.IDEQ(id)).
		First(ctx)
}

// IsNameTaken 校验任务名唯一性。
func (r *BackupTaskRepo) IsNameTaken(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.entClient.Client().BackupTask.Query().Where(backuptask.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(backuptask.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf("query backup task name taken: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *BackupTaskRepo) Create(ctx context.Context, req *opsV1.CreateBackupTaskRequest, operatorID uint32) (*opsV1.BackupTask, error) {
	if req == nil || req.Data == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupTask.Create().
		SetName(req.Data.GetName()).
		SetNodeID(req.Data.GetNodeId()).
		SetSourceDir(req.Data.GetSourceDir()).
		SetCron(req.Data.GetCron()).
		SetRetention(int(req.Data.GetRetention())).
		SetEnabled(req.Data.GetEnabled()).
		SetNillableDescription(req.Data.Description)

	if operatorID > 0 {
		builder.SetCreatedBy(operatorID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert backup task failed: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("insert failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *BackupTaskRepo) Update(ctx context.Context, req *opsV1.UpdateBackupTaskRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupTask.UpdateOneID(req.GetId())
	builder.Where(backuptask.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.BackupTask) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableSourceDir(req.Data.SourceDir).
				SetNillableCron(req.Data.Cron).
				SetNillableDescription(req.Data.Description).
				SetEnabled(req.Data.GetEnabled())

			if req.Data.NodeId != nil && req.Data.GetNodeId() > 0 {
				builder.SetNodeID(req.Data.GetNodeId())
			}
			if req.Data.Retention != nil {
				builder.SetRetention(int(req.Data.GetRetention()))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(backuptask.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *BackupTaskRepo) Delete(ctx context.Context, req *opsV1.DeleteBackupTaskRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().BackupTask.Delete()
	delBuilder.Where(backuptask.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("backup task not found")
		}
		r.log.Errorf("delete backup task [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// AdvanceCronAnchor 定时调度锚点 CAS 推进(多副本去重)。
func (r *BackupTaskRepo) AdvanceCronAnchor(ctx context.Context, id uint32, expected, next time.Time) (bool, error) {
	n, err := r.entClient.Client().BackupTask.
		Update().
		Where(
			backuptask.IDEQ(id),
			backuptask.LastCronAtEQ(expected),
		).
		SetLastCronAt(next).
		Save(ctx)
	return n > 0, err
}

// AdvanceCronAnchorFromNull 给空锚点任务落初始锚点(仅 NULL 行生效)。
func (r *BackupTaskRepo) AdvanceCronAnchorFromNull(ctx context.Context, id uint32, now time.Time) (bool, error) {
	n, err := r.entClient.Client().BackupTask.
		Update().
		Where(
			backuptask.IDEQ(id),
			backuptask.LastCronAtIsNil(),
		).
		SetLastCronAt(now).
		Save(ctx)
	return n > 0, err
}
