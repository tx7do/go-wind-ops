package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// PromotionStage 晋升流水线的单个阶段。
type PromotionStage struct {
	// DistributionID 该阶段执行的分发配置(环境隔离靠分发目标落在对应 env 的节点)。
	DistributionID uint32 `json:"distributionId"`
	// Env 阶段环境标签(展示用,如 dev/staging/prod)。
	Env string `json:"env"`
	// Name 阶段名(如 开发/预发/生产)。
	Name string `json:"name"`
}

// PromotionPipeline 环境晋升流水线:把同一构建产物按有序阶段逐环境晋升。
// 不新造部署路径——阶段即既有分发配置,Promote = 按指定构建历史触发该阶段
// 分发(缺省最新成功构建);跨阶段节奏由人把握,产物可追溯性复用分发记录。
type PromotionPipeline struct {
	ent.Schema
}

func (PromotionPipeline) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_promotion_pipelines",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("环境晋升流水线表"),
	}
}

func (PromotionPipeline) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("流水线名").
			NotEmpty(),

		field.JSON("stages", []PromotionStage{}).
			Comment("有序阶段列表(每阶段=分发配置+环境标签)"),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),
	}
}

func (PromotionPipeline) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}
