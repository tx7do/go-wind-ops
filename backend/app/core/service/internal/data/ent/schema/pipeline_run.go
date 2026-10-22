package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// PipelineRun 流水线运行记录:触发时快照步骤,执行器按序推进;
// 日志与每步结果增量落库,前端可断线重看。
type PipelineRun struct {
	ent.Schema
}

func (PipelineRun) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_pipeline_runs",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("流水线运行记录表"),
	}
}

// Fields of the PipelineRun.
func (PipelineRun) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("pipeline_id").
			Comment("流水线配置(ops_build_pipelines.id)").
			Positive(),

		field.Enum("status").
			Comment("运行状态").
			NamedValues(
				"Pending", "PENDING",
				"Running", "RUNNING",
				"Success", "SUCCESS",
				"Failed", "FAILED",
			).
			Default("PENDING").
			Optional().
			Nillable(),

		field.Text("steps").
			Comment("步骤快照(JSON,触发时定版,配置后改不影响本 run)").
			Optional().
			Nillable(),

		field.Int32("current_step").
			Comment("当前执行到的步骤下标(0 起,-1=未开始)").
			Default(-1),

		field.Text("step_results").
			Comment("每步结果(JSON 数组:type/ref_id/name/status/error/duration)").
			Optional().
			Nillable(),

		field.Text("log").
			Comment("运行日志(步骤横幅+脚本输出,增量落库)").
			Optional().
			Nillable(),

		field.String("error").
			Comment("失败原因(首个失败步骤)").
			Optional().
			Nillable(),

		field.Time("started_at").
			Comment("开始执行时间").
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("结束时间(终态)").
			Optional().
			Nillable(),

		field.Uint32("operator_id").
			Comment("触发人").
			Default(0).
			Optional().
			Nillable(),

		field.String("runner_id").
			Comment("执行实例标识(机器指纹):执行 goroutine 归属实例,启动清扫只定案本实例的遗留记录").
			Default("").
			Optional().
			Nillable(),
	}
}

// Mixin of the PipelineRun.
func (PipelineRun) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
	}
}

// Indexes of the PipelineRun.
func (PipelineRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("pipeline_id").StorageKey("idx_ops_pipeline_run_pipeline"),
		index.Fields("status").StorageKey("idx_ops_pipeline_run_status"),
	}
}
