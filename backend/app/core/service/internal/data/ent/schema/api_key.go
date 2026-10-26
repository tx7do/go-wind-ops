package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ApiKey OpenAPI 密钥:外部系统(CI/CD)经 OpenAPI 端点触发构建/分发/流水线。
// 明文仅创建时返回一次,库内只存 SHA-256;prefix 存前 12 位明文便于识别。
type ApiKey struct {
	ent.Schema
}

func (ApiKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_api_keys",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("OpenAPI 密钥表"),
	}
}

// Fields of the ApiKey.
func (ApiKey) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("密钥名(唯一)").
			Unique().
			NotEmpty(),

		field.String("key_hash").
			Sensitive().
			Comment("SHA-256(opsk_ 明文)").
			Unique().
			NotEmpty(),

		field.String("prefix").
			Comment("明文前 12 位(opsk_xxxxxxxx 识别用)").
			NotEmpty(),

		field.Enum("status").
			Comment("状态").
			NamedValues(
				"Enabled", "ENABLED",
				"Disabled", "DISABLED",
			).
			Default("ENABLED").
			Optional().
			Nillable(),

		field.Time("last_used_at").
			Comment("最近使用时间(异步回写)").
			Optional().
			Nillable(),

		field.Time("expires_at").
			Comment("过期时间(空=永不过期)").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the ApiKey.
func (ApiKey) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the ApiKey.
func (ApiKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key_hash").StorageKey("uidx_ops_api_key_hash"),
	}
}
