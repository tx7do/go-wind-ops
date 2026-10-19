package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// TerminalSession 终端会话审计:每次 Web 终端打开一行,输入/输出分块落
// ops_terminal_chunks 供回放;录制失败不影响终端本身。
type TerminalSession struct {
	ent.Schema
}

func (TerminalSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_terminal_sessions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("终端会话审计表"),
	}
}

// Fields of the TerminalSession.
func (TerminalSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("session_id").
			Comment("会话 ID(agent 侧 UUID,与 MQTT 主题对应)").
			Unique().
			NotEmpty(),

		field.Uint32("node_id").
			Comment("节点(ops_agent_nodes.id)").
			Default(0).
			Optional(),

		field.String("agent_id").
			Comment("机器唯一标识").
			NotEmpty(),

		field.Uint32("operator_id").
			Comment("打开终端的用户").
			Default(0).
			Optional(),

		field.Enum("status").
			Comment("会话状态").
			NamedValues(
				"Open", "OPEN",
				"Closed", "CLOSED",
			).
			Default("OPEN"),

		field.Int64("input_bytes").
			Comment("已录制输入字节数").
			Default(0),

		field.Int64("output_bytes").
			Comment("已录制输出字节数").
			Default(0),

		field.Int("chunk_count").
			Comment("已落库分块数").
			Default(0),

		field.Bool("truncated").
			Comment("是否因超出录制上限被截断").
			Default(false),

		field.Time("closed_at").
			Comment("会话关闭时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the TerminalSession.
func (TerminalSession) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the TerminalSession.
func (TerminalSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id").StorageKey("idx_ops_term_session_node"),
		index.Fields("status").StorageKey("idx_ops_term_session_status"),
	}
}
