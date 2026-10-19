package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Cert 证书到期台账:core 直连目标 host:port 做 TLS 握手读取对端证书,
// 回填签发者/有效期;调度器按提前天数产生到期告警(经通知渠道外呼),
// 续期后剩余天数回升自动恢复。
type Cert struct {
	ent.Schema
}

func (Cert) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_certs",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("证书到期台账表"),
	}
}

// Fields of the Cert.
func (Cert) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("证书名(全局唯一)").
			Unique().
			NotEmpty(),

		field.String("host").
			Comment("探测目标主机(域名/IP)").
			NotEmpty(),

		field.Int("port").
			Comment("TLS 端口").
			Default(443),

		field.String("issuer").
			Comment("签发者(探测回填)").
			Optional().
			Nillable(),

		field.Time("not_before").
			Comment("生效时间(探测回填)").
			Optional().
			Nillable(),

		field.Time("not_after").
			Comment("到期时间(探测回填)").
			Optional().
			Nillable(),

		field.Int("days_before_expire").
			Comment("到期前多少天告警").
			Default(30),

		field.String("probe_cron").
			Comment("自动探测周期 cron(5 段标准语法,空=仅手动)").
			Optional().
			Default(""),

		field.Time("last_probed_at").
			Comment("上次探测时间").
			Optional().
			Nillable(),

		field.String("last_error").
			Comment("上次探测失败原因").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用(到期检查与调度探测)").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the Cert.
func (Cert) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Cert.
func (Cert) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled").StorageKey("idx_ops_cert_enabled"),
	}
}
