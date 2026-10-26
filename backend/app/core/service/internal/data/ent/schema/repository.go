package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Repository 构建源仓库配置(对齐 jpom RepositoryModel 简化版)。
//
// v1 仅支持 HTTPS Git 仓库;密码经全局加密器(AES-256-GCM)落库,
// 回执给前端时脱敏为固定掩码。
type Repository struct {
	ent.Schema
}

func (Repository) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_repositories",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建源仓库表"),
	}
}

// Fields of the Repository.
func (Repository) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("仓库名(全局唯一)").
			Unique().
			NotEmpty(),

		field.String("git_url").
			Comment("Git 仓库地址(v1 仅 HTTPS;本地路径用于开发/测试)").
			NotEmpty(),

		field.String("branch").
			Comment("默认分支").
			Default("main").
			Optional().
			Nillable(),

		field.String("username").
			Comment("仓库用户名(可空)").
			Optional().
			Nillable(),

		field.String("password").
			Comment("仓库密码/令牌(AES-256-GCM 加密落库,查询脱敏)").
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

// Mixin of the Repository.
func (Repository) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Repository.
func (Repository) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_repository_name"),
	}
}
