package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// NotificationChannel 告警通知渠道配置。
// v1 支持 WEBHOOK(含钉钉/飞书/企微群机器人签名)和 EMAIL 两种渠道。
type NotificationChannel struct {
	ent.Schema
}

func (NotificationChannel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_notification_channels",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("告警通知渠道配置表"),
	}
}

func (NotificationChannel) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("渠道名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Enum("type").
			Comment("渠道类型").
			NamedValues(
				"Webhook", "WEBHOOK",
				"Email", "EMAIL",
			).
			Default("WEBHOOK"),

		// ── WEBHOOK 通用 ──
		field.String("webhook_url").
			Comment("Webhook URL").
			Optional().
			Nillable(),
		field.String("webhook_secret").
			Comment("签名密钥(钉钉/飞书/企微群机器人)").
			Sensitive().
			Optional().
			Nillable(),
		field.Enum("webhook_sign_style").
			Comment("签名风格").
			NamedValues(
				"None", "NONE",
				"Dingtalk", "DINGTALK",
				"Feishu", "FEISHU",
				"Wecom", "WECOM",
			).
			Default("NONE").
			Optional().
			Nillable(),

		// ── EMAIL ──
		field.String("smtp_host").
			Comment("SMTP 主机").
			Optional().
			Nillable(),
		field.Int("smtp_port").
			Comment("SMTP 端口").
			Default(587).
			Optional().
			Nillable(),
		field.String("smtp_username").
			Comment("SMTP 用户名").
			Optional().
			Nillable(),
		field.String("smtp_password").
			Comment("SMTP 密码").
			Sensitive().
			Optional().
			Nillable(),
		field.String("smtp_from").
			Comment("发件人地址").
			Optional().
			Nillable(),
		field.String("email_to").
			Comment("收件人地址列表(逗号分隔)").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

func (NotificationChannel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

func (NotificationChannel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_notif_channel_name"),
	}
}
