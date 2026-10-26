package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// NotificationDelivery 告警通知投递台账:每次通过渠道发送告警的通知记录。
type NotificationDelivery struct {
	ent.Schema
}

func (NotificationDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_notification_deliveries",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("告警通知投递台账表"),
	}
}

func (NotificationDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("alert_id").
			Comment("关联告警(ops_alerts.id)").
			Positive(),

		field.Uint32("channel_id").
			Comment("使用的渠道(ops_notification_channels.id)").
			Positive(),

		field.String("channel_type").
			Comment("渠道类型快照(WEBHOOK/EMAIL)").
			NotEmpty(),

		field.String("target").
			Comment("发送目标(URL/邮箱,脱敏存储)").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("发送状态").
			NamedValues(
				"Sending", "SENDING",
				"Sent", "SENT",
				"Failed", "FAILED",
			).
			Default("SENDING"),

		field.String("last_error").
			Comment("发送失败原因").
			Optional().
			Nillable(),

		field.String("title").
			Comment("通知标题").
			Optional().
			Nillable(),

		field.Text("content").
			Comment("通知内容").
			Optional().
			Nillable(),
	}
}

func (NotificationDelivery) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

func (NotificationDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("alert_id").StorageKey("idx_ops_notif_delivery_alert"),
		index.Fields("status").StorageKey("idx_ops_notif_delivery_status"),
	}
}
