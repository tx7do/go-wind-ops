package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ScriptVersion 脚本内容版本快照:UpdateScript 内容变化时由服务层自动
// 落一条旧内容快照,可查看/恢复(恢复也先快照当前,历史永不丢)。
type ScriptVersion struct {
	ent.Schema
}

func (ScriptVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_script_versions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("脚本内容版本历史表"),
	}
}

func (ScriptVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("script_id").
			Comment("所属脚本").
			Positive(),

		field.Text("content").
			Comment("该版本的内容快照"),

		field.String("remark").
			Comment("快照来源(auto=更新前自动快照/restore=恢复前快照)").
			Default("auto"),

		field.Uint32("created_by").
			Comment("触发快照的操作者(0=系统)").
			Optional().
			Nillable(),
	}
}

func (ScriptVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

func (ScriptVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("script_id").StorageKey("idx_ops_script_version_script"),
	}
}
