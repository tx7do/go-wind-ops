package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ConfigTemplate 配置文件模板:集中管理按环境的配置内容,下发时经 MinIO
// 中转 + file_push 指令写到目标节点路径(agent 侧授权目录校验)。
type ConfigTemplate struct {
	ent.Schema
}

func (ConfigTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_config_templates",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("配置文件模板表"),
	}
}

// Fields of the ConfigTemplate.
func (ConfigTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("模板名(全局唯一)").
			Unique().
			NotEmpty(),

		field.String("env").
			Comment("所属环境(default/dev/staging/prod)").
			Default("default"),

		field.String("target_path").
			Comment("下发目标绝对路径(须在 agent 授权目录内)").
			NotEmpty(),

		field.Text("content").
			Comment("模板内容(≤256KB,原样下发)").
			Optional(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the ConfigTemplate.
func (ConfigTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the ConfigTemplate.
func (ConfigTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("env").StorageKey("idx_ops_config_tpl_env"),
	}
}
