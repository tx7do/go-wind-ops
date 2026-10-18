package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Distribution 构建产物分发配置(对齐 jpom OutGiving 简化版):
// 把某构建配置的产物投递到目标项目目录,v1 为单项目目标。
type Distribution struct {
	ent.Schema
}

func (Distribution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_distributions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建产物分发配置表"),
	}
}

// Fields of the Distribution.
func (Distribution) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("分发名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("build_id").
			Comment("构建来源(ops_builds.id);触发时取其最新成功产物").
			Positive(),

		field.Uint32("project_id").
			Comment("目标项目(ops_projects.id)").
			Positive(),

		field.Bool("clear_old").
			Comment("分发前清空项目目录旧内容(危险操作,谨慎开启)").
			Default(false),

		field.Enum("after_opt").
			Comment("分发后动作").
			NamedValues(
				"None", "NONE",
				"Restart", "RESTART",
				"Start", "START",
			).
			Default("NONE").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("标签").
			Optional(),
	}
}

// Mixin of the Distribution.
func (Distribution) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Distribution.
func (Distribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_distribution_name"),
		index.Fields("build_id").StorageKey("idx_ops_distribution_build"),
		index.Fields("project_id").StorageKey("idx_ops_distribution_project"),
	}
}
