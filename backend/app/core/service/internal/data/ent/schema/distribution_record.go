package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// DistributionRecord 一次分发执行的记录:引用构建历史与指令回执,
// 状态机 PENDING → SUCCESS/FAILED(由 dist_apply 指令回执驱动)。
type DistributionRecord struct {
	ent.Schema
}

func (DistributionRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_distribution_records",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("分发执行记录表"),
	}
}

// Fields of the DistributionRecord.
func (DistributionRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("distribution_id").
			Comment("分发配置(ops_distributions.id)").
			Positive(),

		field.Uint32("build_history_id").
			Comment("构建历史(ops_build_histories.id)").
			Positive(),

		field.Uint32("project_id").
			Comment("目标项目(ops_projects.id)").
			Positive(),

		field.Uint32("node_id").
			Comment("目标节点(ops_agent_nodes.id)").
			Positive(),

		field.String("command_request_id").
			Comment("关联的 MQTT 指令幂等键(dist_apply)").
			NotEmpty(),

		field.Enum("status").
			Comment("分发状态").
			NamedValues(
				"Pending", "PENDING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING").
			Optional().
			Nillable(),

		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.Text("result").
			Comment("执行结果数据(JSON:部署文件数/输出等)").
			Optional().
			Nillable(),

		field.Uint32("operator_id").
			Comment("触发人").
			Default(0).
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("结束时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the DistributionRecord.
func (DistributionRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the DistributionRecord.
func (DistributionRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("distribution_id").StorageKey("idx_ops_dist_record_dist"),
		index.Fields("command_request_id").StorageKey("uidx_ops_dist_record_req"),
		index.Fields("status").StorageKey("idx_ops_dist_record_status"),
	}
}
