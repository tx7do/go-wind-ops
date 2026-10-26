package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Monitor 监控规则(对齐 jpom MonitorModel 简化版):周期检查某个项目的
// 运行状态、某台节点的资源指标或某台节点上的进程存在性,异常时产生告警
// 并可自动重启。
// 检查由 core 的调度器按 cron 周期驱动:项目规则经 MQTT 同步指令探测,
// 节点规则直接读心跳指标快照(ops_agent_nodes 行上字段),进程规则经
// process_list 同步指令探测。
type Monitor struct {
	ent.Schema
}

func (Monitor) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_monitors",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("监控规则表"),
	}
}

// Fields of the Monitor.
func (Monitor) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("规则名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("project_id").
			Comment("被监控项目(ops_projects.id;节点指标规则为 0)").
			Default(0).
			Optional(),

		field.Enum("target_type").
			Comment("规则目标类型:PROJECT=项目状态 / NODE_METRIC=节点资源指标 / PROCESS=进程存在性").
			NamedValues(
				"Project", "PROJECT",
				"NodeMetric", "NODE_METRIC",
				"Process", "PROCESS",
			).
			Default("PROJECT"),

		field.Uint32("node_id").
			Comment("被监控节点(ops_agent_nodes.id;项目状态规则为 0)").
			Default(0).
			Optional(),

		field.String("process_name").
			Comment("被监控进程名/关键字(进程规则用;大小写不敏感包含匹配)").
			Default("").
			Optional(),

		field.Enum("metric_type").
			Comment("指标类型(节点指标规则用):CPU/MEMORY/DISK").
			NamedValues(
				"Cpu", "CPU",
				"Memory", "MEMORY",
				"Disk", "DISK",
			).
			Default("CPU"),

		field.Float("threshold_percent").
			Comment("告警阈值(百分比,指标 ≥ 阈值即命中)").
			Default(0),

		field.Int("consecutive_count").
			Comment("连续命中多少个检查周期才告警(防抖)").
			Default(1),

		field.Int("consecutive_hits").
			Comment("运行态:当前连续命中次数(调度器维护,非用户配置)").
			Default(0),

		field.Bool("notify_on_recovery").
			Comment("恢复时是否产生 RECOVERY 告警并通知").
			Default(true),

		field.Int("repeat_interval_minutes").
			Comment("重复告警最小间隔(分钟,0=每次异常检查都报;恢复后重新告警同样受此约束)").
			Default(0),

		field.String("cron").
			Comment("检查周期 cron 表达式(5 段标准语法)").
			NotEmpty(),

		field.Time("last_check_at").
			Comment("检查调度锚点(调度器 CAS 推进,多副本去重;NULL=首见落锚不补跑)").
			Optional().
			Nillable(),

		field.Bool("auto_restart").
			Comment("异常时自动重启项目(仅项目状态规则生效)").
			Default(false),

		field.String("webhook_url").
			Comment("告警通知 webhook 地址(空则只落告警不外呼)").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

// Mixin of the Monitor.
func (Monitor) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Monitor.
func (Monitor) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").StorageKey("idx_ops_monitor_project"),
		index.Fields("node_id").StorageKey("idx_ops_monitor_node"),
		index.Fields("enabled").StorageKey("idx_ops_monitor_enabled"),
	}
}
