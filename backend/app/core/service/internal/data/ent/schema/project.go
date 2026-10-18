package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Project 运行在受管机器上的应用项目(jpom 的 NodeProjectInfo 简化版)。
//
// v1 采用通用命令型模型:不区分 Java/Node 等运行模式,以启动/停止/状态
// 三条命令描述生命周期,由 agent 在项目目录内执行。project_dir 必须落在
// agent 的授权目录(whitelist)内,agent 执行前会二次校验。
type Project struct {
	ent.Schema
}

func (Project) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_projects",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("受管机器应用项目表"),
	}
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("node_id").
			Comment("所属节点(ops_agent_nodes.id)").
			Positive(),

		field.String("name").
			Comment("项目名(节点内唯一)").
			NotEmpty(),

		field.String("description").
			Comment("项目描述").
			Optional().
			Nillable(),

		field.String("project_dir").
			Comment("项目目录(受管机器绝对路径,须在 agent 授权目录内)").
			NotEmpty(),

		field.String("start_command").
			Comment("启动命令(在项目目录内以 shell 执行,长驻进程由 agent 记 pid)").
			Optional().
			Nillable(),

		field.String("stop_command").
			Comment("停止命令(优先于 pid 终止执行)").
			Optional().
			Nillable(),

		field.String("status_command").
			Comment("状态命令(输出作为状态判定依据;缺省按 pid 存活判断)").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("标签").
			Optional(),

		field.Uint64("last_pid").
			Comment("最近一次启动记录的进程 pid(agent 回填)").
			Default(0).
			Optional().
			Nillable(),

		field.Enum("last_status").
			Comment("最近上报的运行状态").
			NamedValues(
				"Unknown", "UNKNOWN",
				"Running", "RUNNING",
				"Stopped", "STOPPED",
			).
			Default("UNKNOWN").
			Optional().
			Nillable(),

		field.Time("last_status_at").
			Comment("状态最近更新时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the Project.
func (Project) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Project.
func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_id", "name").Unique().
			StorageKey("uidx_ops_project_node_name"),
		index.Fields("node_id").StorageKey("idx_ops_project_node"),
	}
}
