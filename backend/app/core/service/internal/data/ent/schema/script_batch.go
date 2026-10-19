package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// ScriptBatch 脚本批量执行批次:一次把脚本下发到多台节点,逐节点生成
// 指令台账(ops_command_records.batch_id 关联),回执驱动计数与终态。
type ScriptBatch struct {
	ent.Schema
}

func (ScriptBatch) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_script_batches",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("脚本批量执行批次表"),
	}
}

// Fields of the ScriptBatch.
func (ScriptBatch) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("script_id").
			Comment("脚本(ops_scripts.id)").
			Positive(),

		field.String("script_name").
			Comment("脚本名快照(脚本改名/删除不影响批次展示)").
			NotEmpty(),

		field.JSON("node_ids", []uint32{}).
			Comment("目标节点 ID 列表").
			Optional(),

		field.Uint32("total").
			Comment("目标节点总数").
			Default(0),

		field.Uint32("success_count").
			Comment("已成功节点数(回执驱动累加)").
			Default(0),

		field.Uint32("failed_count").
			Comment("已失败节点数(回执/下发失败/超时定案累加)").
			Default(0),

		field.Enum("status").
			Comment("批次状态:RUNNING=进行中 / FINISHED=全部终态").
			NamedValues(
				"Running", "RUNNING",
				"Finished", "FINISHED",
			).
			Default("RUNNING"),
	}
}

// Mixin of the ScriptBatch.
func (ScriptBatch) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the ScriptBatch.
func (ScriptBatch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("script_id").StorageKey("idx_ops_script_batch_script"),
		index.Fields("status").StorageKey("idx_ops_script_batch_status"),
	}
}
