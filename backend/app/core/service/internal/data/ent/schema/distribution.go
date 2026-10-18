package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// Distribution 构建产物分发配置(对齐 jpom OutGiving 简化版):
// 把某构建配置的产物投递到目标项目目录,v1 为单项目目标。
type Distribution struct {
	ent.Schema
}

func (Distribution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_distributions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("构建产物分发配置表"),
	}
}

// Fields of the Distribution.
func (Distribution) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("分发名(全局唯一)").
			Unique().
			NotEmpty(),

		field.Uint32("build_id").
			Comment("构建来源(ops_builds.id);触发时取其最新成功产物").
			Positive(),

		field.JSON("project_ids", []uint32{}).
			Comment("目标项目 ID 列表(支持多目标,逐项目触发分发记录)").
			Optional(),

		field.Bool("clear_old").
			Comment("分发前清空项目目录旧内容(危险操作,谨慎开启)").
			Default(false),

		field.Enum("deploy_type").
			Comment("部署类型:FILE=产物文件分发,DOCKER=镜像部署(目标机需有 docker)").
			NamedValues(
				"File", "FILE",
				"Docker", "DOCKER",
			).
			Default("FILE").
			Optional().
			Nillable(),

		field.String("image").
			Comment("镜像 tag(DOCKER 用,支持 {buildNo} 占位符)").
			Optional().
			Nillable(),

		field.String("container_name").
			Comment("容器名(DOCKER 用;部署时先强制删除同名容器再 run)").
			Optional().
			Nillable(),

		field.JSON("docker_ports", []string{}).
			Comment("端口映射(DOCKER 用,host:container)").
			Optional(),

		field.JSON("docker_envs", []string{}).
			Comment("环境变量(DOCKER 用,K=V)").
			Optional(),

		field.JSON("docker_volumes", []string{}).
			Comment("卷挂载(DOCKER 用,host:container)").
			Optional(),

		field.String("docker_restart").
			Comment("容器重启策略(DOCKER 用)").
			Default("unless-stopped").
			Optional().
			Nillable(),

		field.String("docker_network").
			Comment("容器网络(DOCKER 用)").
			Optional().
			Nillable(),

		field.Enum("after_opt").
			Comment("分发后动作").
			NamedValues(
				"None", "NONE",
				"Restart", "RESTART",
				"Start", "START",
			).
			Default("NONE").
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

// Mixin of the Distribution.
func (Distribution) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

// Indexes of the Distribution.
func (Distribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").StorageKey("uidx_ops_distribution_name"),
		index.Fields("build_id").StorageKey("idx_ops_distribution_build"),
		index.Fields("project_ids").StorageKey("idx_ops_distribution_projects"),
	}
}
