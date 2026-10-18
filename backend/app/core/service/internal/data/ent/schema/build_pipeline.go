package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// BuildPipeline 构建流水线:有序步骤(构建/脚本/分发)按序执行、
// 失败即中止。步骤以 JSON 文本存储(服务层与 proto PipelineStep 互转)。
type BuildPipeline struct {
	ent.Schema
}

func (BuildPipeline) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_build_pipelines",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建流水线配置表"),
	}
}

// Fields of the BuildPipeline.
func (BuildPipeline) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("流水线名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Text("steps").
			Comment("步骤列表(JSON 数组:type=BUILD/SCRIPT/DISTRIBUTION,ref_id,agent_id,name)"),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("标签").
			Optional(),
	}
}

// Mixin of the BuildPipeline.
func (BuildPipeline) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the BuildPipeline.
func (BuildPipeline) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_pipeline_name"),
	}
}
