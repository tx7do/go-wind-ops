// Package data 中 AgentNodeRepo:agent 节点(机器资产)的仓储。
// 标准 CRUD 走 entCrud 泛型仓储(与 task_repo 同构);MQTT 信令链路相关的
// 特殊查询(准入/心跳/离线扫描)为手写窄接口,直用 ent client。
package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"
	"entgo.io/ent/dialect/sql"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/agentnode"
	"go-wind-ops/app/core/service/internal/data/ent/predicate"
	"go-wind-ops/pkg/signal"

	opsV1 "go-wind-ops/api/gen/go/ops/service/v1"
)

type AgentNodeRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper        *mapper.CopierMapper[opsV1.AgentNode, ent.AgentNode]
	typeConverter *mapper.EnumTypeConverter[opsV1.AgentNodeStatus, agentnode.Status]

	repository *entCrud.Repository[
		ent.AgentNodeQuery, ent.AgentNodeSelect,
		ent.AgentNodeCreate, ent.AgentNodeCreateBulk,
		ent.AgentNodeUpdate, ent.AgentNodeUpdateOne,
		ent.AgentNodeDelete,
		predicate.AgentNode,
		opsV1.AgentNode, ent.AgentNode,
	]
}

func NewAgentNodeRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AgentNodeRepo {
	repo := &AgentNodeRepo{
		log:           ctx.NewLoggerHelper("agent-node/repo/core-service"),
		entClient:     entClient,
		mapper:        mapper.NewCopierMapper[opsV1.AgentNode, ent.AgentNode](),
		typeConverter: mapper.NewEnumTypeConverter[opsV1.AgentNodeStatus, agentnode.Status](opsV1.AgentNodeStatus_name, opsV1.AgentNodeStatus_value),
	}

	repo.init()

	return repo
}

func (r *AgentNodeRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.AgentNodeQuery, ent.AgentNodeSelect,
		ent.AgentNodeCreate, ent.AgentNodeCreateBulk,
		ent.AgentNodeUpdate, ent.AgentNodeUpdateOne,
		ent.AgentNodeDelete,
		predicate.AgentNode,
		opsV1.AgentNode, ent.AgentNode,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

// ============================================================================
// 标准 CRUD(供 gRPC AgentNodeService 使用)
// ============================================================================

func (r *AgentNodeRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().AgentNode.Query()
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

func (r *AgentNodeRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*opsV1.ListAgentNodeResponse, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AgentNode.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &opsV1.ListAgentNodeResponse{Total: 0, Items: nil}, nil
	}

	return &opsV1.ListAgentNodeResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *AgentNodeRepo) Get(ctx context.Context, req *opsV1.GetAgentNodeRequest) (*opsV1.AgentNode, error) {
	if req == nil {
		return nil, opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AgentNode.Query()

	dto, err := r.repository.Get(ctx, builder, nil, agentnode.IDEQ(req.GetId()))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, opsV1.ErrorNotFound("agent node not found")
		}
		r.log.Errorf("query agent node [%d]: %s", req.GetId(), err.Error())
		return nil, opsV1.ErrorInternalServerError("query failed")
	}

	return dto, nil
}

func (r *AgentNodeRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().AgentNode.Query().
		Where(agentnode.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, opsV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Update 仅允许运维侧字段(name/labels);机器指纹与在线状态由 agent 链路维护。
func (r *AgentNodeRepo) Update(ctx context.Context, req *opsV1.UpdateAgentNodeRequest) error {
	if req == nil || req.Data == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AgentNode.UpdateOneID(req.GetId())
	builder.Where(agentnode.IDEQ(req.GetId()))

	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *opsV1.AgentNode) {
			builder.
				SetNillableName(req.Data.Name)

			if len(req.Data.Labels) > 0 {
				builder.SetLabels(req.Data.Labels)
			}

			builder.SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(agentnode.FieldID, req.GetId()))
		},
	)

	return err
}

func (r *AgentNodeRepo) Delete(ctx context.Context, req *opsV1.DeleteAgentNodeRequest) error {
	if req == nil {
		return opsV1.ErrorBadRequest("invalid parameter")
	}

	delBuilder := r.entClient.Client().AgentNode.Delete()
	delBuilder.Where(agentnode.IDEQ(req.GetId()))
	if _, err := delBuilder.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return opsV1.ErrorNotFound("agent node not found")
		}
		r.log.Errorf("delete agent node [%d]: %s", req.GetId(), err.Error())
		return opsV1.ErrorInternalServerError("delete failed")
	}

	return nil
}

// ============================================================================
// MQTT 信令链路特殊查询
// ============================================================================

// GetByAgentID 按机器唯一标识查询节点;不存在返回 (nil, nil)。
func (r *AgentNodeRepo) GetByAgentID(ctx context.Context, agentID string) (*ent.AgentNode, error) {
	node, err := r.entClient.Client().AgentNode.
		Query().
		Where(agentnode.AgentID(agentID)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		r.log.Errorf("query agent node [%s]: %s", agentID, err.Error())
		return nil, err
	}
	return node, nil
}

// Enrol TOFU 首连注册:落库令牌哈希与机器指纹,初始在线。
func (r *AgentNodeRepo) Enrol(ctx context.Context, agentID, tokenHash string, info *signal.AgentInfo) (*ent.AgentNode, error) {
	name := agentID
	if info != nil && info.Hostname != "" {
		name = info.Hostname
	}
	builder := r.entClient.Client().AgentNode.
		Create().
		SetAgentID(agentID).
		SetTokenHash(tokenHash).
		SetName(name).
		SetStatus(agentnode.StatusOnline).
		SetNillableLastHeartbeatAt(nowPtr())

	applyInfo(builder.Mutation(), info)

	node, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("enrol agent node [%s]: %s", agentID, err.Error())
		return nil, err
	}
	return node, nil
}

// UpdateAuthSuccess 准入通过:刷新指纹并置在线。
func (r *AgentNodeRepo) UpdateAuthSuccess(ctx context.Context, nodeID uint32, info *signal.AgentInfo) error {
	builder := r.entClient.Client().AgentNode.
		UpdateOneID(nodeID).
		SetStatus(agentnode.StatusOnline).
		SetNillableLastHeartbeatAt(nowPtr())
	applyInfo(builder.Mutation(), info)
	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf("update agent node [%d] auth success: %s", nodeID, err.Error())
		return err
	}
	return nil
}

// TouchHeartbeat 心跳到达:刷新在线状态/最后心跳时间/指标快照。
func (r *AgentNodeRepo) TouchHeartbeat(ctx context.Context, agentID string, metrics *signal.AgentMetrics) error {
	builder := r.entClient.Client().AgentNode.
		Update().
		Where(agentnode.AgentID(agentID)).
		SetStatus(agentnode.StatusOnline).
		SetNillableLastHeartbeatAt(nowPtr())
	if metrics != nil {
		builder.
			SetNillableCPUPercent(ptr[float64](metrics.CPUPercent)).
			SetNillableMemoryUsedPercent(ptr[float64](metrics.MemoryUsedPercent)).
			SetNillableDiskUsedPercent(ptr[float64](metrics.DiskUsedPercent)).
			SetNillableUptimeSeconds(ptr[uint64](metrics.UptimeSeconds))
	}
	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf("touch heartbeat [%s]: %s", agentID, err.Error())
		return err
	}
	return nil
}

// MarkStaleOffline 把最后心跳早于 before 的在线节点标记离线,返回影响行数。
// 幂等,可多副本并发执行。
func (r *AgentNodeRepo) MarkStaleOffline(ctx context.Context, before time.Time) (int, error) {
	n, err := r.entClient.Client().AgentNode.
		Update().
		Where(
			agentnode.StatusEQ(agentnode.StatusOnline),
			agentnode.LastHeartbeatAtLT(before),
		).
		SetStatus(agentnode.StatusOffline).
		Save(ctx)
	if err != nil {
		r.log.Errorf("mark stale agents offline: %s", err.Error())
		return 0, err
	}
	return n, nil
}

// CountByStatus 返回在线/离线节点数(运维概览用)。
func (r *AgentNodeRepo) CountByStatus(ctx context.Context) (online, offline int, err error) {
	online, err = r.entClient.Client().AgentNode.
		Query().
		Where(agentnode.StatusEQ(agentnode.StatusOnline)).
		Count(ctx)
	if err != nil {
		return 0, 0, err
	}
	offline, err = r.entClient.Client().AgentNode.
		Query().
		Where(agentnode.StatusEQ(agentnode.StatusOffline)).
		Count(ctx)
	if err != nil {
		return 0, 0, err
	}
	return online, offline, nil
}

// applyInfo 把机器指纹写入 mutation(空值跳过,不覆盖已有值)。
func applyInfo(m *ent.AgentNodeMutation, info *signal.AgentInfo) {
	if info == nil {
		return
	}
	if info.Hostname != "" {
		m.SetHostname(info.Hostname)
	}
	if info.OS != "" {
		m.SetOs(info.OS)
	}
	if info.Arch != "" {
		m.SetArch(info.Arch)
	}
	if info.Platform != "" {
		m.SetPlatform(info.Platform)
	}
	if info.PlatformVersion != "" {
		m.SetPlatformVersion(info.PlatformVersion)
	}
	if info.KernelVersion != "" {
		m.SetKernelVersion(info.KernelVersion)
	}
	if info.AgentVersion != "" {
		m.SetAgentVersion(info.AgentVersion)
	}
	if len(info.IPs) > 0 {
		m.SetIps(info.IPs)
	}
	if info.CPUCores > 0 {
		m.SetCPUCores(info.CPUCores)
	}
	if info.MemTotalBytes > 0 {
		m.SetMemTotalBytes(info.MemTotalBytes)
	}
	if len(info.Labels) > 0 {
		m.SetLabels(info.Labels)
	}
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

func ptr[T any](v T) *T { return &v }
