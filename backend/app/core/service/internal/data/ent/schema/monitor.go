package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Monitor 监控规则(对齐 jpom MonitorModel 简化版):周期检查某个项目的
// 运行状态,异常时产生告警并可自动重启。
// 检查由 core 的调度器按 cron 周期经 MQTT 同步指令探测。
type Monitor struct {
	ent.Schema
}

func (Monitor) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_monitors",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("监控规则表"),
	}
}

// Fields of the Monitor.
func (Monitor) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("规则名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("project_id").
			Comment("被监控项目(ops_projects.id)").
			Positive(),

		field.String("cron").
			Comment("检查周期 cron 表达式(5 段标准语法)").
			NotEmpty(),

		field.Bool("auto_restart").
			Comment("异常时自动重启项目").
			Default(false),

		field.String("webhook_url").
			Comment("告警通知 webhook 地址(空则只落告警不外呼)").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the Monitor.
func (Monitor) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Monitor.
func (Monitor) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").StorageKey("idx_ops_monitor_project"),
		index.Fields("enabled").StorageKey("idx_ops_monitor_enabled"),
	}
}
