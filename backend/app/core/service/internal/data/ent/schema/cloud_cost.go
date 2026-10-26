package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// CloudCost 云成本月账单:账号 × 月份一条,手工录入或 CSV 导入。
// 定位是"轻量成本台账"(回答每月各云厂商花多少),不对接计费 API。
type CloudCost struct {
	ent.Schema
}

func (CloudCost) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_cloud_costs",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("云成本月账单表"),
	}
}

// Fields of the CloudCost.
func (CloudCost) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("cloud_account_id").
			Comment("云账号ID").
			Positive(),

		field.String("cloud_account_name").
			Comment("云账号名(快照)").
			Optional().
			Nillable(),

		field.Enum("provider").
			Comment("云厂商(录入时快照,汇总按此分组)").
			NamedValues(
				"Aliyun", "ALIYUN",
				"TencentCloud", "TENCENT_CLOUD",
				"HuaweiCloud", "HUAWEI_CLOUD",
				"AWS", "AWS",
			).
			Optional().
			Nillable(),

		field.String("month").
			Comment("账单月份 YYYY-MM").
			NotEmpty(),

		field.Float("amount").
			Comment("月度成本金额"),

		field.String("currency").
			Comment("币种(默认 CNY;汇总不换汇,多币种各自展示)").
			Default("CNY").
			NotEmpty(),

		field.String("remark").
			Comment("备注").
			Optional().
			Nillable(),
	}
}

// Mixin of the CloudCost.
func (CloudCost) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the CloudCost.
func (CloudCost) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("cloud_account_id", "month").Unique().StorageKey("uidx_ops_cloud_cost_key"),
		index.Fields("month").StorageKey("idx_ops_cloud_cost_month"),
	}
}
