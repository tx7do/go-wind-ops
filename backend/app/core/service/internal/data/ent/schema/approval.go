package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Approval 操作审批单:危险操作(生产分发/脚本执行)需审批人确认后才执行。
// 操作发起时创建 PENDING 单,审批通过后由系统执行实际操作。
type Approval struct {
	ent.Schema
}

func (Approval) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_approvals",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("操作审批单表"),
	}
}

// Fields of the Approval.
func (Approval) Fields() []ent.Field {
	return []ent.Field{
		field.String("operation_type").
			Comment("操作类型(distribute/script_exec)").
			NotEmpty(),

		field.Uint32("operation_ref_id").
			Comment("操作关联实体 ID(分发配置 ID 或脚本 ID)").
			Positive(),

		field.String("agent_id").
			Comment("目标节点 agentId(审批通过后据此重新下发指令)").
			NotEmpty(),

		field.String("operation_name").
			Comment("操作名称(快照,便于审批人识别)").
			NotEmpty(),

		field.Text("payload").
			Comment("操作载荷快照(JSON,审批通过后按此执行)").
			Optional().
			Nillable(),

		field.Uint32("requester_id").
			Comment("发起人").
			Default(0).
			Optional().
			Nillable(),

		field.Uint32("approver_id").
			Comment("审批人").
			Default(0).
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("审批状态").
			NamedValues(
				"Pending", "PENDING",
				"Approved", "APPROVED",
				"Rejected", "REJECTED",
			).
			Default("PENDING"),

		field.String("reject_reason").
			Comment("驳回原因").
			Optional().
			Nillable(),

		field.Text("result").
			Comment("执行结果数据(JSON)").
			Optional().
			Nillable(),

		field.Time("resolved_at").
			Comment("审批时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the Approval.
func (Approval) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

// Indexes of the Approval.
func (Approval) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").StorageKey("idx_ops_approval_status"),
		index.Fields("requester_id").StorageKey("idx_ops_approval_requester"),
	}
}
