package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// CloudAccount 云平台账号:多云环境的 AK/SK 台账,供在线校验与后续资源/账单对接。
// SK 落库经 EncryptIfNeeded 加密,接口永不回传明文。
type CloudAccount struct {
	ent.Schema
}

func (CloudAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_cloud_accounts",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("云平台账号表"),
	}
}

// Fields of the CloudAccount.
func (CloudAccount) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("账号别名(唯一)").
			Unique().
			NotEmpty(),

		field.Enum("provider").
			Comment("云厂商").
			NamedValues(
				"Aliyun", "ALIYUN",
				"TencentCloud", "TENCENT_CLOUD",
				"HuaweiCloud", "HUAWEI_CLOUD",
				"AWS", "AWS",
			).
			Optional().
			Nillable(),

		field.String("account_id").
			Comment("云平台账号ID(在线校验后回填)").
			Optional().
			Nillable(),

		field.String("account_name").
			Comment("云平台账号名(在线校验后回填)").
			Optional().
			Nillable(),

		field.String("access_key").
			Comment("访问密钥 AK").
			Optional().
			Nillable(),

		field.String("secret_key").
			Sensitive().
			Comment("安全密钥 SK(加密存储)").
			Optional().
			Nillable(),

		field.String("region").
			Comment("主地域(如 cn-hangzhou / us-east-1)").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("状态").
			NamedValues(
				"Enabled", "ENABLED",
				"Disabled", "DISABLED",
			).
			Default("ENABLED").
			Optional().
			Nillable(),

		field.Time("last_verified_at").
			Comment("最近在线校验时间").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the CloudAccount.
func (CloudAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}
