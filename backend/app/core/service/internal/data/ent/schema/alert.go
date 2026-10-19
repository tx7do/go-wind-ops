package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Alert 告警记录:监控检查异常时产生(项目状态/节点指标两类规则),
// 恢复时可自动产生 RECOVERY 记录,人工确认后标记已恢复。
type Alert struct {
	ent.Schema
}

func (Alert) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_alerts",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("告警记录表"),
	}
}

// Fields of the Alert.
func (Alert) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("monitor_id").
			Comment("来源监控规则(ops_monitors.id;预算/证书等非监控来源为 0)").
			Default(0).
			Optional(),

		field.String("source").
			Comment("告警来源:MONITOR=监控规则 / BUDGET=云成本预算 / CERT=证书到期").
			Default("MONITOR"),

		field.Uint32("project_id").
			Comment("关联项目(节点指标/预算/证书告警为 0)").
			Default(0).
			Optional(),

		field.Uint32("node_id").
			Comment("关联节点(ops_agent_nodes.id;项目状态/预算/证书告警为 0)").
			Default(0).
			Optional(),

		field.Enum("level").
			Comment("告警级别").
			NamedValues(
				"Warning", "WARNING",
				"Error", "ERROR",
				"Recovery", "RECOVERY",
			).
			Default("ERROR"),

		field.String("message").
			Comment("告警内容").
			NotEmpty(),

		field.Bool("resolved").
			Comment("是否已恢复/确认").
			Default(false),

		field.Time("resolved_at").
			Comment("恢复时间").
			Optional().
			Nillable(),

		field.Bool("notified").
			Comment("是否已外呼 webhook").
			Default(false),
	}
}

// Mixin of the Alert.
func (Alert) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the Alert.
func (Alert) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("monitor_id").StorageKey("idx_ops_alert_monitor"),
		index.Fields("node_id").StorageKey("idx_ops_alert_node"),
		index.Fields("resolved").StorageKey("idx_ops_alert_resolved"),
	}
}
