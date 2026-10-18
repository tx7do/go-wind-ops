package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// NodeMetric 节点心跳指标时间序列(每次心跳追加一行)。
// 保留策略:由调度器定期清理(默认保留 7 天)。
type NodeMetric struct {
	ent.Schema
}

func (NodeMetric) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_node_metrics",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("节点心跳指标时间序列"),
	}
}

// Fields of the NodeMetric.
func (NodeMetric) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("node_id").
			Comment("节点(ops_agent_nodes.id)").
			Positive(),

		field.Float("cpu_percent").
			Comment("CPU 占用百分比").
			Optional().
			Nillable(),

		field.Float("memory_used_percent").
			Comment("内存占用百分比").
			Optional().
			Nillable(),

		field.Float("disk_used_percent").
			Comment("磁盘占用百分比").
			Optional().
			Nillable(),

		field.Uint64("mem_total_bytes").
			Comment("物理内存总量").
			Default(0).
			Optional().
			Nillable(),

		field.Uint64("uptime_seconds").
			Comment("系统运行时长(秒)").
			Default(0).
			Optional().
			Nillable(),
	}
}

// Mixin of the NodeMetric.
func (NodeMetric) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the NodeMetric.
func (NodeMetric) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id", "created_at").StorageKey("idx_ops_node_metric_node_time"),
	}
}
