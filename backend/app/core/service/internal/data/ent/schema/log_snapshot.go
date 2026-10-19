package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// LogSnapshot 项目日志快照:按需(后续可定时)把项目运行日志尾部拉回
// 平台落库,支持跨项目关键字检索;上限 256KB/份。
type LogSnapshot struct {
	ent.Schema
}

func (LogSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_log_snapshots",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("项目日志快照表"),
	}
}

// Fields of the LogSnapshot.
func (LogSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("project_id").
			Comment("项目(ops_projects.id)").
			Positive(),

		field.Uint32("node_id").
			Comment("执行节点(ops_agent_nodes.id)").
			Positive(),

		field.String("project_name").
			Comment("项目名快照").
			NotEmpty(),

		field.Int64("size_bytes").
			Comment("日志快照大小(字节)").
			Default(0),

		field.Text("content").
			Comment("日志内容(尾部,≤256KB)").
			Optional(),

		field.String("trigger").
			Comment("触发方式:MANUAL=手动 / SCHEDULED=定时").
			Default("MANUAL"),
	}
}

// Mixin of the LogSnapshot.
func (LogSnapshot) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the LogSnapshot.
func (LogSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").StorageKey("idx_ops_log_snapshot_project"),
	}
}
