package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// CommandRecord 经 MQTT 下发给 agent 的指令台账(审计+结果回执落库)。
//
// core 的 PublishCommand 创建 PENDING 行,HandleCommandResult 按
// request_id 回填终态;超时未回执的行由离线扫描一并定案 FAILED。
type CommandRecord struct {
	ent.Schema
}

func (CommandRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_command_records",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("Agent 指令台账表"),
	}
}

// Fields of the CommandRecord.
func (CommandRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").
			Comment("幂等键(UUID),与 CommandEnvelope/CommandResult 对应").
			Unique().
			NotEmpty(),

		field.Uint32("node_id").
			Comment("目标节点(ops_agent_nodes.id);离线注入等无记录场景为 0").
			Default(0).
			Optional().
			Nillable(),

		field.String("agent_id").
			Comment("目标机器唯一标识").
			NotEmpty(),

		field.String("action").
			Comment("指令动作(ping/project_start/project_stop/project_status/...)").
			NotEmpty(),

		field.Text("payload").
			Comment("指令载荷(JSON)").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("执行状态").
			NamedValues(
				"Pending", "PENDING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING").
			Optional().
			Nillable(),
		field.Text("result").
			Comment("执行结果数据(JSON)").
			Optional().
			Nillable(),
		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.Uint32("operator_id").
			Comment("触发指令的用户").
			Default(0).
			Optional().
			Nillable(),

		field.Uint32("batch_id").
			Comment("所属脚本批量执行批次(ops_script_batches.id;非批量指令为 0)").
			Default(0).
			Optional(),

		field.Time("finished_at").
			Comment("结果回执时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the CommandRecord.
func (CommandRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

// Indexes of the CommandRecord.
func (CommandRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id").StorageKey("idx_ops_cmd_node"),
		index.Fields("status").StorageKey("idx_ops_cmd_status"),
		index.Fields("batch_id").StorageKey("idx_ops_cmd_batch"),
		index.Fields("agent_id", "action").StorageKey("idx_ops_cmd_agent_action"),
	}
}
