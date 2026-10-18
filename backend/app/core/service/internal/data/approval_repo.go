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
	"go-wind-ops/app/core/service/internal/data/ent/approval"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type ApprovalRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.Approval, ent.Approval]

	repository *entCrud.Repository[
		ent.ApprovalQuery, ent.ApprovalSelect,
		ent.ApprovalCreate, ent.ApprovalCreateBulk,
		ent.ApprovalUpdate, ent.ApprovalUpdateOne,
		ent.ApprovalDelete,
		predicate.Approval,
		opsV1.Approval, ent.Approval,
	]
}

func NewApprovalRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ApprovalRepo {
	repo := &ApprovalRepo{
		log:       ctx.NewLoggerHelper("approval/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.Approval, ent.Approval](),
	}

	repo.init()

	return repo
}

func (r *ApprovalRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ApprovalQuery, ent.ApprovalSelect,
		ent.ApprovalCreate, ent.ApprovalCreateBulk,
		ent.ApprovalUpdate, ent.ApprovalUpdateOne,
		ent.ApprovalDelete,
		predicate.Approval,
		opsV1.Approval, ent.Approval,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *ApprovalRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().Approval.Query()
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

func (r *ApprovalRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListApprovalResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Approval.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListApprovalResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListApprovalResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ApprovalRepo) Get(ctx context.Context, req *opsV1.GetApprovalRequest) (*opsV1.Approval, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Approval.Query()

	dto, err := r.repository.Get(ctx, builder, nil, approval.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("approval not found")
		}
		r.log.Errorf("query approval [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

// GetEntity 取原始实体(审批通过后执行操作用)。
func (r *ApprovalRepo) GetEntity(ctx context.Context, id uint32) (*ent.Approval, error) {
	return r.entClient.Client().Approval.
		Query().
		Where(approval.IDEQ(id)).
		First(ctx)
}

// Create 创建审批单(PENDING)。operationType 为字符串(distribute/script_exec)。
func (r *ApprovalRepo) Create(ctx context.Context, operationType string, operationRefID uint32, agentID, opName, payload string, requesterID uint32) (*ent.Approval, error) {
	builder := r.entClient.Client().Approval.
		Create().
		SetOperationType(operationType).
		SetOperationRefID(operationRefID).
		SetAgentID(agentID).
		SetOperationName(opName).
		SetStatus(approval.StatusPending)

	if payload != "" {
		builder.SetPayload(payload)
	}
	if requesterID > 0 {
		builder.SetRequesterID(requesterID)
	}

	record, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create approval: %s", err.Error())
		return nil, opsV1.ErrorInternalServerError("create approval failed")
	}
	return record, nil
}

// Resolve 审批(通过/驳回)。仅 PENDING 可审批(CAS 去重)。
func (r *ApprovalRepo) Resolve(ctx context.Context, id uint32, approve bool, approverID uint32, rejectReason string) error {
	now := time.Now()
	status := approval.StatusRejected
	if approve {
		status = approval.StatusApproved
	}

	builder := r.entClient.Client().Approval.
		UpdateOneID(id).
		Where(approval.StatusEQ(approval.StatusPending)).
		SetStatus(status).
		SetApproverID(approverID).
		SetNillableResolvedAt(&now)

	if !approve && rejectReason != "" {
		builder.SetRejectReason(rejectReason)
	}

	n, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("resolve approval [%d]: %s", id, err.Error())
		return opsV1.ErrorInternalServerError("resolve failed")
	}
	_ = n
	return nil
}

// MarkExecuted 审批通过的操作执行完毕后回写结果。
func (r *ApprovalRepo) MarkExecuted(ctx context.Context, id uint32, result string) error {
	_, err := r.entClient.Client().Approval.
		UpdateOneID(id).
		SetResult(result).
		Save(ctx)
	return err
}
