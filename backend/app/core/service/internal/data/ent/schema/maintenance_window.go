package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// MaintenanceWindow 告警维护窗口:计划停机/发布窗口期静默告警通知。
// 平台级实体(无租户列):后台任务(cron/离线扫描)无 viewer 上下文也要能
// 做抑制判定;窗口属全局运维状态,与 agent_node 同款平台级建模。
// 语义:窗口激活期间告警行仍落库(留痕可追溯),仅外呼通知被跳过;
// 恢复通知不受窗口影响(维护期恢复更需可知)。
type MaintenanceWindow struct {
	ent.Schema
}

func (MaintenanceWindow) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_maintenance_windows",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("告警维护窗口表"),
	}
}

func (MaintenanceWindow) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("窗口名(如:周六发布停机)").
			NotEmpty(),

		field.Time("starts_at").
			Comment("开始时间"),

		field.Time("ends_at").
			Comment("结束时间(>= starts_at,服务层校验)"),

		field.Bool("enabled").
			Comment("是否启用(停用即失效,不必删)").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

func (MaintenanceWindow) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

func (MaintenanceWindow) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "starts_at").StorageKey("idx_ops_maint_window_active"),
	}
}
