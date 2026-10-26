package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Script 运维脚本(对齐 jpom 的 ScriptModel 简化版)。
//
// 脚本内容集中存放在 core,执行时经 MQTT 指令下发到目标节点(agent 落地
// 临时文件后以平台 shell 执行)。v1 不做定时执行与 webhook 触发器。
type Script struct {
	ent.Schema
}

func (Script) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_scripts",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("运维脚本表"),
	}
}

// Fields of the Script.
func (Script) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("脚本名(全局唯一)").
			Unique().
			NotEmpty(),

		field.String("description").
			Comment("脚本描述").
			Optional().
			Nillable(),

		field.Text("content").
			Comment("脚本内容(按目标节点平台 shell 语法编写)"),

		field.Uint32("timeout_seconds").
			Comment("执行超时(秒,默认 60,上限 600)").
			Default(60).
			Optional().
			Nillable(),

		field.String("cron").
			Comment("定时执行 cron 表达式(5 段标准语法,空为手动触发)").
			Optional().
			Nillable(),

		field.Uint32("cron_node_id").
			Comment("定时执行的目标节点(0 为未配置,不调度)").
			Default(0).
			Optional().
			Nillable(),

		field.Time("last_cron_at").
			Comment("上次定时调度锚点(调度器去重用)").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("标签").
			Optional(),
	}
}

// Mixin of the Script.
func (Script) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Script.
func (Script) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_script_name"),
	}
}
