package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// BackupTask 备份任务:把某台节点上授权目录内的目录打包 zip,经 MinIO
// 预签名 URL 直传对象存储;支持 cron 定时与保留策略(超出保留数的成功
// 备份按时间清理)。执行明细见 ops_backup_records。
type BackupTask struct {
	ent.Schema
}

func (BackupTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_backup_tasks",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("备份任务表"),
	}
}

// Fields of the BackupTask.
func (BackupTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("任务名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("node_id").
			Comment("目标节点(ops_agent_nodes.id)").
			Positive(),

		field.String("source_dir").
			Comment("备份源目录(须在 agent 授权目录 AGENT_ALLOWED_DIRS 内)").
			NotEmpty(),

		field.String("cron").
			Comment("定时备份 cron 表达式(5 段标准语法,空为仅手动触发)").
			Optional().
			Default(""),

		field.Int("retention").
			Comment("保留最近 N 份成功备份(0=全部保留)").
			Default(0),

		field.Bool("enabled").
			Comment("是否启用(停用后不再参与定时调度,手动触发不受限)").
			Default(true),

		field.Time("last_cron_at").
			Comment("上次定时调度锚点(调度器去重用)").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the BackupTask.
func (BackupTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the BackupTask.
func (BackupTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id").StorageKey("idx_ops_backup_task_node"),
		index.Fields("enabled").StorageKey("idx_ops_backup_task_enabled"),
	}
}
