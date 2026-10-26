package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/nodemetric"
)

// NodeMetricRepo 节点心跳指标时间序列(写入+按时间段查询+清理)。
type NodeMetricRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewNodeMetricRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *NodeMetricRepo {
	return &NodeMetricRepo{
		entClient: entClient,
		log:       ctx.NewLoggerHelper("node-metric/repo/core-service"),
	}
}

// Create 每次心跳追加一条指标。
func (r *NodeMetricRepo) Create(ctx context.Context, nodeID uint32, cpu, mem, disk float64, memTotal, uptime uint64) error {
	_, err := r.entClient.Client().NodeMetric.
		Create().
		SetNodeID(nodeID).
		SetNillableCPUPercent(f64Ptr(cpu)).
		SetNillableMemoryUsedPercent(f64Ptr(mem)).
		SetNillableDiskUsedPercent(f64Ptr(disk)).
		SetNillableMemTotalBytes(u64Ptr(memTotal)).
		SetNillableUptimeSeconds(u64Ptr(uptime)).
		Save(ctx)
	if err != nil {
		r.log.Errorf("create node metric [%d]: %s", nodeID, err.Error())
	}
	return err
}

// MetricPoint 一次心跳的指标数据点。
type MetricPoint struct {
	Time              time.Time `json:"time"`
	CPUPercent        float64   `json:"cpu_percent"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	DiskUsedPercent   float64   `json:"disk_used_percent"`
}

// ListByNodeAndTimeRange 按节点和时间范围查指标(时间正序)。
func (r *NodeMetricRepo) ListByNodeAndTimeRange(ctx context.Context, nodeID uint32, from, to time.Time) ([]MetricPoint, error) {
	rows, err := r.entClient.Client().NodeMetric.
		Query().
		Where(
			nodemetric.NodeIDEQ(nodeID),
			nodemetric.CreatedAtGTE(from),
			nodemetric.CreatedAtLTE(to),
		).
		Order(ent.Asc(nodemetric.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		r.log.Errorf("query node metrics [%d]: %s", nodeID, err.Error())
		return nil, err
	}
	out := make([]MetricPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, MetricPoint{
			Time:              derefT(row.CreatedAt),
			CPUPercent:        derefF(row.CPUPercent),
			MemoryUsedPercent: derefF(row.MemoryUsedPercent),
			DiskUsedPercent:   derefF(row.DiskUsedPercent),
		})
	}
	return out, nil
}

// CleanupOlderThan 删除指定时间之前的指标(保留策略)。
func (r *NodeMetricRepo) CleanupOlderThan(ctx context.Context, before time.Time) (int, error) {
	n, err := r.entClient.Client().NodeMetric.
		Delete().
		Where(nodemetric.CreatedAtLT(before)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("cleanup node metrics: %s", err.Error())
	}
	return n, err
}

func f64Ptr(v float64) *float64 { return &v }
func u64Ptr(v uint64) *uint64   { return &v }
func derefF(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefT(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}
