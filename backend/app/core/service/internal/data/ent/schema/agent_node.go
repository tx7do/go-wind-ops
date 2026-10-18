package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// AgentNode 受管机器上的 agent 节点(运维平台的"机器资产"实体,
// 对应 jpom 的 MachineNode/NodeModel)。
//
// 平台级实体,无租户列;agent 经 MQTT 准入后写入,心跳刷新在线状态。
type AgentNode struct {
	ent.Schema
}

func (AgentNode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_agent_nodes",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("Agent 节点表"),
	}
}

// Fields of the AgentNode.
func (AgentNode) Fields() []ent.Field {
	return []ent.Field{
		field.String("agent_id").
			Comment("机器唯一标识(machine-id),MQTT 主题与准入的锚点").
			Unique().
			NotEmpty(),

		field.String("name").
			Comment("节点显示名(默认取 hostname,可人工修改)").
			Optional().
			Nillable(),

		field.String("token_hash").
			Comment("接入令牌的 SHA-256 十六进制哈希(明文只存 agent 本机)").
			NotEmpty(),

		field.Enum("status").
			Comment("在线状态").
			NamedValues(
				"Online", "ONLINE",
				"Offline", "OFFLINE",
			).
			Default("OFFLINE").
			Optional().
			Nillable(),

		field.String("env").
			Comment("环境标签(default/dev/staging/prod,登记时指定,用于环境隔离)").
			Default("default").
			Optional().
			Nillable(),

		field.String("hostname").
			Comment("主机名").
			Optional().
			Nillable(),

		field.String("os").
			Comment("操作系统(linux/windows/darwin)").
			Optional().
			Nillable(),

		field.String("arch").
			Comment("CPU 架构(amd64/arm64/...)").
			Optional().
			Nillable(),

		field.String("platform").
			Comment("发行版/系统名称(ubuntu/windows 10...)").
			Optional().
			Nillable(),

		field.String("platform_version").
			Comment("系统版本").
			Optional().
			Nillable(),

		field.String("kernel_version").
			Comment("内核版本").
			Optional().
			Nillable(),

		field.String("agent_version").
			Comment("agent 进程版本").
			Optional().
			Nillable(),

		field.JSON("ips", []string{}).
			Comment("非环回 IP 列表").
			Optional(),

		field.Int("cpu_cores").
			Comment("逻辑核数").
			Optional().
			Nillable(),

		field.Uint64("mem_total_bytes").
			Comment("物理内存总量").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("人工标签(业务轮用于分组/筛选)").
			Optional(),

		field.Float("cpu_percent").
			Comment("最新 CPU 占用百分比(心跳快照)").
			Optional().
			Nillable(),

		field.Float("memory_used_percent").
			Comment("最新内存占用百分比(心跳快照)").
			Optional().
			Nillable(),

		field.Float("disk_used_percent").
			Comment("最新磁盘占用百分比(心跳快照)").
			Optional().
			Nillable(),

		field.Uint64("uptime_seconds").
			Comment("系统运行时长(秒,心跳快照)").
			Optional().
			Nillable(),

		field.Time("last_heartbeat_at").
			Comment("最后心跳时间").
			Optional().
			Nillable(),
	}
}

// Mixin of the AgentNode.
func (AgentNode) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the AgentNode.
func (AgentNode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status").StorageKey("idx_ops_agent_node_status"),
	}
}
