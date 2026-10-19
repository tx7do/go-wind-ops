package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/backuprecord"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type BackupRecordRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.BackupRecord, ent.BackupRecord]

	repository *entCrud.Repository[
		ent.BackupRecordQuery, ent.BackupRecordSelect,
		ent.BackupRecordCreate, ent.BackupRecordCreateBulk,
		ent.BackupRecordUpdate, ent.BackupRecordUpdateOne,
		ent.BackupRecordDelete,
		predicate.BackupRecord,
		opsV1.BackupRecord, ent.BackupRecord,
	]
}

func NewBackupRecordRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BackupRecordRepo {
	repo := &BackupRecordRepo{
		log:       ctx.NewLoggerHelper("backup-record/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.BackupRecord, ent.BackupRecord](),
	}

	repo.init()

	return repo
}

func (r *BackupRecordRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.BackupRecordQuery, ent.BackupRecordSelect,
		ent.BackupRecordCreate, ent.BackupRecordCreateBulk,
		ent.BackupRecordUpdate, ent.BackupRecordUpdateOne,
		ent.BackupRecordDelete,
		predicate.BackupRecord,
		opsV1.BackupRecord, ent.BackupRecord,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *BackupRecordRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListBackupRecordResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupRecord.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListBackupRecordResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListBackupRecordResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *BackupRecordRepo) Get(ctx context.Context, req *opsV1.GetBackupRecordDownloadUrlRequest) (*opsV1.BackupRecord, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().BackupRecord.Query()

	dto, err := r.repository.Get(ctx, builder, nil, backuprecord.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("backup record not found")
		}
		r.log.Errorf("query backup record [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// Create 创建 PENDING 备份记录(指令下发成功后调用)。
func (r *BackupRecordRepo) Create(ctx context.Context, taskID, nodeID uint32, bucket, objectKey, requestID string, operatorID uint32) (*ent.BackupRecord, error) {
	builder := r.entClient.Client().BackupRecord.
		Create().
		SetTaskID(taskID).
		SetNodeID(nodeID).
		SetBucket(bucket).
		SetObjectKey(objectKey).
		SetCommandRequestID(requestID).
		SetStatus(backuprecord.StatusPending)
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create backup record: %s", err.Error())
		return nil, err
	}
	return record, nil
}

// FinalizeByCommandRequestID 按指令幂等键回填终态 + 大小/校验和;仅
// PENDING 行生效(重复回执/超时定案幂等)。
func (r *BackupRecordRepo) FinalizeByCommandRequestID(ctx context.Context, requestID, status, errMsg string, sizeBytes int64, sha256 string) error {
	now := time.Now()
	builder := r.entClient.Client().BackupRecord.
		Update().
		Where(
			backuprecord.CommandRequestIDEQ(requestID),
			backuprecord.StatusEQ(backuprecord.StatusPending),
		).
		SetStatus(backuprecord.Status(status)).
		SetSizeBytes(sizeBytes).
		SetNillableError(ptrOrNil(errMsg)).
		SetNillableFinishedAt(&now)
	if sha256 != "" {
		builder.SetSha256(sha256)
	}
	_, err := builder.Save(ctx)
	return err
}

// FinalizeStalePending 把超时未回执的 PENDING 记录定案 FAILED。幂等,可多副本并发。
func (r *BackupRecordRepo) FinalizeStalePending(ctx context.Context, before time.Time) (int, error) {
	now := time.Now()
	n, err := r.entClient.Client().BackupRecord.
		Update().
		Where(
			backuprecord.StatusEQ(backuprecord.StatusPending),
			backuprecord.CreatedAtLT(before),
		).
		SetStatus(backuprecord.StatusFailed).
		SetError("backup timeout without result").
		SetNillableFinishedAt(&now).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ListSuccessHoldersByTask 取某任务按时间倒序的第 keep 份之后的成功记录
// (保留策略清扫用)。
func (r *BackupRecordRepo) ListSuccessHoldersByTask(ctx context.Context, taskID uint32, keep int) ([]*ent.BackupRecord, error) {
	query := r.entClient.Client().BackupRecord.
		Query().
		Where(
			backuprecord.TaskIDEQ(taskID),
			backuprecord.StatusEQ(backuprecord.StatusSuccess),
		).
		Order(ent.Desc(backuprecord.FieldCreatedAt))
	if keep > 0 {
		query = query.Offset(keep)
	}
	return query.All(ctx)
}

// DeleteEntity 删除单条记录(保留策略清扫用)。
func (r *BackupRecordRepo) DeleteEntity(ctx context.Context, id uint32) error {
	return r.entClient.Client().BackupRecord.
		DeleteOneID(id).
		Exec(ctx)
}
