package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// TerminalChunk 终端会话录制分块:base64 承载原始字节(Windows GBK 回显
// 不失真),按 (会话, seq) 有序回放。
type TerminalChunk struct {
	ent.Schema
}

func (TerminalChunk) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_terminal_chunks",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("终端会话录制分块表"),
	}
}

// Fields of the TerminalChunk.
func (TerminalChunk) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("terminal_session_id").
			Comment("会话行(ops_terminal_sessions.id)").
			Positive(),

		field.Int("seq").
			Comment("会话内序号(录制器分配,从 0 递增)"),

		field.Enum("direction").
			Comment("方向").
			NamedValues(
				"Input", "INPUT",
				"Output", "OUTPUT",
			).
			Default("OUTPUT"),

		field.Text("payload").
			Comment("分块内容(base64)"),

		field.Int("byte_len").
			Comment("原始字节数").
			Default(0),
	}
}

// Mixin of the TerminalChunk.
func (TerminalChunk) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the TerminalChunk.
func (TerminalChunk) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("terminal_session_id", "seq").Unique().StorageKey("uidx_ops_term_chunk_seq"),
	}
}
