package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// CloudBudget 云成本月度预算:按账号(0=全部账号合计)+月份设预算,调度器
// 周期把台账成本与预算比对,达到阈值或环比突增时产生告警并通知渠道。
type CloudBudget struct {
	ent.Schema
}

func (CloudBudget) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_cloud_budgets",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("云成本月度预算表"),
	}
}

// Fields of the CloudBudget.
func (CloudBudget) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("cloud_account_id").
			Comment("云账号(ops_cloud_accounts.id;0=全部账号合计)").
			Default(0).
			Optional(),

		field.String("month").
			Comment("预算月份(YYYY-MM)").
			NotEmpty(),

		field.Float("amount").
			Comment("预算金额").
			Default(0),

		field.String("currency").
			Comment("币种(CNY/USD/...)").
			Default("CNY"),

		field.Float("threshold_percent").
			Comment("达到预算的百分比时告警(默认 100)").
			Default(100),

		field.Bool("notified").
			Comment("本月阈值告警是否已发(防重复)").
			Default(false),

		field.Float("anomaly_ratio").
			Comment("环比突增倍数(本月成本>前 3 月均值×该倍数时告警;0=不检测)").
			Default(0),

		field.Bool("anomaly_notified").
			Comment("本月突增告警是否已发(防重复)").
			Default(false),

		field.Bool("enabled").
			Comment("是否启用检查").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the CloudBudget.
func (CloudBudget) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the CloudBudget.
func (CloudBudget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("cloud_account_id", "month").Unique().StorageKey("uidx_ops_cloud_budget_key"),
		index.Fields("enabled").StorageKey("idx_ops_cloud_budget_enabled"),
	}
}
