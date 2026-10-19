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
	"go-wind-ops/app/core/service/internal/data/ent/predicate"
	"go-wind-ops/app/core/service/internal/data/ent/terminalchunk"
	"go-wind-ops/app/core/service/internal/data/ent/terminalsession"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

// TerminalSessionRepo 终端会话审计仓储。
type TerminalSessionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[opsV1.TerminalSession, ent.TerminalSession]

	repository *entCrud.Repository[
		ent.TerminalSessionQuery, ent.TerminalSessionSelect,
		ent.TerminalSessionCreate, ent.TerminalSessionCreateBulk,
		ent.TerminalSessionUpdate, ent.TerminalSessionUpdateOne,
		ent.TerminalSessionDelete,
		predicate.TerminalSession,
		opsV1.TerminalSession, ent.TerminalSession,
	]
}

func NewTerminalSessionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *TerminalSessionRepo {
	repo := &TerminalSessionRepo{
		log:       ctx.NewLoggerHelper("terminal-session/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[opsV1.TerminalSession, ent.TerminalSession](),
	}

	repo.init()

	return repo
}

func (r *TerminalSessionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.TerminalSessionQuery, ent.TerminalSessionSelect,
		ent.TerminalSessionCreate, ent.TerminalSessionCreateBulk,
		ent.TerminalSessionUpdate, ent.TerminalSessionUpdateOne,
		ent.TerminalSessionDelete,
		predicate.TerminalSession,
		opsV1.TerminalSession, ent.TerminalSession,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *TerminalSessionRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListTerminalSessionResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().TerminalSession.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListTerminalSessionResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListTerminalSessionResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// GetEntity 取原始实体(录制器用)。
func (r *TerminalSessionRepo) GetEntity(ctx context.Context, id uint32) (*ent.TerminalSession, error) {
	return r.entClient.Client().TerminalSession.
		Query().
		Where(terminalsession.IDEQ(id)).
		First(ctx)
}

// Create 开会话落一行(OPEN)。
func (r *TerminalSessionRepo) Create(ctx context.Context, sessionId, agentId string, nodeID, operatorID uint32) (*ent.TerminalSession, error) {
	builder := r.entClient.Client().TerminalSession.
		Create().
		SetSessionID(sessionId).
		SetAgentID(agentId).
		SetStatus(terminalsession.StatusOpen)
	if nodeID > 0 {
		builder.SetNodeID(nodeID)
	}
	if operatorID > 0 {
		builder.SetOperatorID(operatorID)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create terminal session: %s", err.Error())
		return nil, err
	}
	return row, nil
}

// Finalize 会话关闭:累加流量/分块计数并置 CLOSED(幂等,仅 OPEN 行生效)。
func (r *TerminalSessionRepo) Finalize(ctx context.Context, id uint32, inputBytes, outputBytes int64, chunkCount int, truncated bool) error {
	now := time.Now()
	_, err := r.entClient.Client().TerminalSession.
		Update().
		Where(
			terminalsession.IDEQ(id),
			terminalsession.StatusEQ(terminalsession.StatusOpen),
		).
		SetStatus(terminalsession.StatusClosed).
		SetInputBytes(inputBytes).
		SetOutputBytes(outputBytes).
		SetChunkCount(chunkCount).
		SetTruncated(truncated).
		SetNillableClosedAt(&now).
		Save(ctx)
	return err
}

// ListExpiredSessionIDs 取已关闭且早于 cutoff 的会话行 ID(留存清理用,
// 分批上限 limit)。
func (r *TerminalSessionRepo) ListExpiredSessionIDs(ctx context.Context, cutoff time.Time, limit int) ([]uint32, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.entClient.Client().TerminalSession.
		Query().
		Where(
			terminalsession.StatusEQ(terminalsession.StatusClosed),
			terminalsession.ClosedAtLT(cutoff),
		).
		Select(terminalsession.FieldID).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

// DeleteByIDs 删除会话行(留存清理;调用方须先删干净关联分块)。
func (r *TerminalSessionRepo) DeleteByIDs(ctx context.Context, ids []uint32) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.entClient.Client().TerminalSession.
		Delete().
		Where(terminalsession.IDIn(ids...)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete terminal sessions: %s", err.Error())
	}
	return err
}

// TerminalChunkRepo 终端录制分块仓储(基表读写,无 DTO 映射)。
type TerminalChunkRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewTerminalChunkRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *TerminalChunkRepo {
	return &TerminalChunkRepo{
		log:       ctx.NewLoggerHelper("terminal-chunk/repo/core-service"),
		entClient: entClient,
	}
}

// CreateBulk 批量落分块(recorder 批量冲刷)。
func (r *TerminalChunkRepo) CreateBulk(ctx context.Context, rows []*ent.TerminalChunk) error {
	if len(rows) == 0 {
		return nil
	}
	builders := make([]*ent.TerminalChunkCreate, 0, len(rows))
	for _, row := range rows {
		builders = append(builders, r.entClient.Client().TerminalChunk.
			Create().
			SetTerminalSessionID(row.TerminalSessionID).
			SetSeq(row.Seq).
			SetDirection(row.Direction).
			SetPayload(row.Payload).
			SetByteLen(row.ByteLen))
	}
	if _, err := r.entClient.Client().TerminalChunk.CreateBulk(builders...).Save(ctx); err != nil {
		r.log.Errorf("create terminal chunks: %s", err.Error())
		return err
	}
	return nil
}

// DeleteBySessionIDs 批量删除会话分块(留存清理,先于会话行删除)。
func (r *TerminalChunkRepo) DeleteBySessionIDs(ctx context.Context, sessionRowIDs []uint32) error {
	if len(sessionRowIDs) == 0 {
		return nil
	}
	_, err := r.entClient.Client().TerminalChunk.
		Delete().
		Where(terminalchunk.TerminalSessionIDIn(sessionRowIDs...)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete terminal chunks: %s", err.Error())
	}
	return err
}

// ListBySession 按会话取分块(基表查询,手写转换——payload 用 bytes 类型)。
func (r *TerminalChunkRepo) ListBySession(ctx context.Context, sessionRowID uint32, fromSeq, limit int) ([]*ent.TerminalChunk, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	return r.entClient.Client().TerminalChunk.
		Query().
		Where(
			terminalchunk.TerminalSessionIDEQ(sessionRowID),
			terminalchunk.SeqGTE(fromSeq),
		).
		Order(ent.Asc(terminalchunk.FieldSeq)).
		Limit(limit + 1).
		All(ctx)
}
