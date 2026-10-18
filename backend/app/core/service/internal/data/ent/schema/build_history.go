package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// BuildHistory 一次构建的历史记录(jpom BuildHistoryLog 简化版):
// 状态机 PENDING → RUNNING → SUCCESS/FAILED;日志全文与产物对象键落库,
// 产物文件在 MinIO(builds/{buildId}/{buildNo}/...)。
type BuildHistory struct {
	ent.Schema
}

func (BuildHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_build_histories",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建历史表"),
	}
}

// Fields of the BuildHistory.
func (BuildHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("build_id").
			Comment("构建配置(ops_builds.id)").
			Positive(),

		field.Uint32("build_no").
			Comment("构建号(配置内自增)").
			Positive(),

		field.Enum("status").
			Comment("构建状态").
			NamedValues(
				"Pending", "PENDING",
				"Running", "RUNNING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING").
			Optional().
			Nillable(),

		field.String("branch").
			Comment("实际构建分支").
			Optional().
			Nillable(),

		field.Text("log").
			Comment("构建日志(克隆+构建输出,截断)").
			Optional().
			Nillable(),

		field.String("artifact_bucket").
			Comment("产物桶").
			Optional().
			Nillable(),

		field.String("artifact_object_key").
			Comment("产物对象键(MinIO)").
			Optional().
			Nillable(),

		field.Uint64("artifact_size").
			Comment("产物大小(字节)").
			Default(0).
			Optional().
			Nillable(),

		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.Time("started_at").
			Comment("开始执行时间").
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("结束时间").
			Optional().
			Nillable(),

		field.Uint32("operator_id").
			Comment("触发人").
			Default(0).
			Optional().
			Nillable(),
	}
}

// Mixin of the BuildHistory.
func (BuildHistory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the BuildHistory.
func (BuildHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("build_id", "build_no").Unique().
			StorageKey("uidx_ops_build_history_build_no"),
		index.Fields("status").StorageKey("idx_ops_build_history_status"),
	}
}
