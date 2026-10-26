package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// BackupRecord 备份执行记录:一次 backup_run 指令对应一行,PENDING 起步,
// agent 回执(MQTT result)驱动终态;对象落 MinIO backups 桶,浏览器经
// 预签名 URL 下载。
type BackupRecord struct {
	ent.Schema
}

func (BackupRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_backup_records",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("备份执行记录表"),
	}
}

// Fields of the BackupRecord.
func (BackupRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("task_id").
			Comment("来源备份任务(ops_backup_tasks.id)").
			Positive(),

		field.Uint32("node_id").
			Comment("执行节点(ops_agent_nodes.id)").
			Positive(),

		field.String("bucket").
			Comment("对象存储桶").
			NotEmpty(),

		field.String("object_key").
			Comment("对象键(backups/{taskId}/{uuid}.zip)").
			NotEmpty(),

		field.Int64("size_bytes").
			Comment("备份包大小(字节,回执回填)").
			Default(0),

		field.String("sha256").
			Comment("备份包 SHA-256(回执回填)").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("执行状态").
			NamedValues(
				"Pending", "PENDING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING"),

		field.String("error").
			Comment("失败原因").
			Optional().
			Nillable(),

		field.String("command_request_id").
			Comment("指令幂等键(与 ops_command_records.request_id 对应)").
			NotEmpty(),

		field.Uint32("operator_id").
			Comment("触发备份的用户(定时触发为 0)").
			Default(0).
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("终态时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the BackupRecord.
func (BackupRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

// Indexes of the BackupRecord.
func (BackupRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id").StorageKey("idx_ops_backup_record_task"),
		index.Fields("status").StorageKey("idx_ops_backup_record_status"),
		index.Fields("command_request_id").Unique().StorageKey("uidx_ops_backup_record_req"),
	}
}
