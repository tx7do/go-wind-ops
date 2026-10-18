package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Build 构建配置(对齐 jpom BuildInfoModel 简化版):绑定源仓库,给出构建
// 命令与产物路径,构建号自增;每次触发生成一条 BuildHistory。
// v1 在 core 所在机器上构建(经典模式);产物打包上传 MinIO。
type Build struct {
	ent.Schema
}

func (Build) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_builds",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建配置表"),
	}
}

// Fields of the Build.
func (Build) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("构建名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("repository_id").
			Comment("源仓库(ops_repositories.id)").
			Positive(),

		field.String("branch").
			Comment("构建分支(缺省用仓库默认分支)").
			Optional().
			Nillable(),

		field.String("build_command").
			Comment("构建命令(在仓库工作目录内以 shell 执行)").
			NotEmpty(),

		field.String("artifact_path").
			Comment("产物路径(相对仓库目录,支持 glob;多个匹配打包 zip)").
			NotEmpty(),

		field.Uint32("timeout_seconds").
			Comment("构建超时(秒,默认 600,上限 3600)").
			Default(600).
			Optional().
			Nillable(),

		field.Uint32("build_no").
			Comment("最新构建号(触发时自增)").
			Default(0),

		field.String("cron").
			Comment("定时构建 cron 表达式(5 段标准语法,空为手动触发)").
			Optional().
			Nillable(),

		field.Time("last_cron_at").
			Comment("上次定时调度锚点(调度器去重用)").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),

		field.JSON("labels", []string{}).
			Comment("标签").
			Optional(),
	}
}

// Mixin of the Build.
func (Build) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Build.
func (Build) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("repository_id").StorageKey("idx_ops_build_repo"),
	}
}
