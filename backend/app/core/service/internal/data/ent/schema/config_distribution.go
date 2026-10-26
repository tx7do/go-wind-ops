package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ConfigDistribution 配置下发记录:一次 file_push 指令对应一行,PENDING
// 起步,agent 回执驱动终态(复用分发记录定案模式)。
type ConfigDistribution struct {
	ent.Schema
}

func (ConfigDistribution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_config_distributions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("配置下发记录表"),
	}
}

// Fields of the ConfigDistribution.
func (ConfigDistribution) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("template_id").
			Comment("来源模板(ops_config_templates.id)").
			Positive(),

		field.String("template_name").
			Comment("模板名快照").
			NotEmpty(),

		field.Uint32("node_id").
			Comment("目标节点(ops_agent_nodes.id)").
			Positive(),

		field.Uint32("project_id").
			Comment("关联项目(ops_projects.id;不关联为 0)").
			Default(0).
			Optional(),

		field.String("target_path").
			Comment("下发目标路径").
			NotEmpty(),

		field.Enum("status").
			Comment("下发状态").
			NamedValues(
				"Pending", "PENDING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING"),

		field.String("command_request_id").
			Comment("指令幂等键(ops_command_records.request_id)").
			NotEmpty(),

		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.Uint32("operator_id").
			Comment("触发下发的用户").
			Default(0).
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("终态时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the ConfigDistribution.
func (ConfigDistribution) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

// Indexes of the ConfigDistribution.
func (ConfigDistribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("template_id").StorageKey("idx_ops_config_dist_tpl"),
		index.Fields("status").StorageKey("idx_ops_config_dist_status"),
		index.Fields("command_request_id").Unique().StorageKey("uidx_ops_config_dist_req"),
	}
}
