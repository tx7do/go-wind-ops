package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Inspection 平台巡检报告:聚合节点/项目/告警/证书/备份等域的当前状态
// 生成一次健康快照(JSON 存 summary),前端渲染并可导出。
type Inspection struct {
	ent.Schema
}

func (Inspection) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_inspections",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("平台巡检报告表"),
	}
}

// Fields of the Inspection.
func (Inspection) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("status").
			Comment("执行状态").
			NamedValues(
				"Running", "RUNNING",
				"Finished", "FINISHED",
				"Failed", "FAILED",
			).
			Default("RUNNING"),

		field.Text("summary").
			Comment("报告 JSON(节点/项目/告警/证书/备份汇总)").
			Optional().
			Nillable(),

		field.Int("node_total").
			Comment("节点总数").
			Default(0),

		field.Int("node_online").
			Comment("在线节点数").
			Default(0),

		field.Int("project_total").
			Comment("项目总数").
			Default(0),

		field.Int("project_running").
			Comment("运行中项目数").
			Default(0),

		field.Int("alert_open").
			Comment("未恢复告警数").
			Default(0),

		field.Int("cert_expiring").
			Comment("30 天内到期证书数").
			Default(0),

		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("完成时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the Inspection.
func (Inspection) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Inspection.
func (Inspection) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").StorageKey("idx_ops_inspection_status"),
	}
}
