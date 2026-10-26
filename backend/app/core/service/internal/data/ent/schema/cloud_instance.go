package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// CloudInstance 云资源实例清单:按云账号同步的全地域实例快照,
// 与 agent_node 按 IP 对账(matched/agent_node_id)。
// 同步语义为整账号全量替换,synced_at 为批次时间。
type CloudInstance struct {
	ent.Schema
}

func (CloudInstance) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ops_cloud_instances",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("云资源实例清单表"),
	}
}

// Fields of the CloudInstance.
func (CloudInstance) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("cloud_account_id").
			Comment("云账号ID").
			Positive(),

		field.String("cloud_account_name").
			Comment("云账号名(快照,账号改名不回溯)").
			Optional().
			Nillable(),

		field.Enum("provider").
			Comment("云厂商(冗余自账号,便于过滤)").
			NamedValues(
				"Aliyun", "ALIYUN",
				"TencentCloud", "TENCENT_CLOUD",
				"HuaweiCloud", "HUAWEI_CLOUD",
				"AWS", "AWS",
			).
			Optional().
			Nillable(),

		field.String("region").
			Comment("地域").
			NotEmpty(),

		field.String("zone").
			Comment("可用区").
			Optional().
			Nillable(),

		field.String("instance_id").
			Comment("云实例ID").
			NotEmpty(),

		field.String("instance_name").
			Comment("实例名称").
			Optional().
			Nillable(),

		field.String("status").
			Comment("实例状态(Running/Stopped/...原样存厂商值)").
			Optional().
			Nillable(),

		field.String("instance_type").
			Comment("机型").
			Optional().
			Nillable(),

		field.String("private_ip").
			Comment("内网IP").
			Optional().
			Nillable(),

		field.String("public_ip").
			Comment("公网IP").
			Optional().
			Nillable(),

		field.Time("expired_at").
			Comment("包年包月到期时间(AWS 按量无此概念为空)").
			Optional().
			Nillable(),

		field.Bool("matched").
			Comment("是否匹配到 agent 节点").
			Default(false),

		field.Uint32("agent_node_id").
			Comment("匹配到的节点ID(快照,节点删除后不回填)").
			Optional().
			Nillable(),

		field.String("agent_node_name").
			Comment("匹配到的节点名(快照)").
			Optional().
			Nillable(),

		field.Time("synced_at").
			Comment("同步批次时间").
			Nillable(),
	}
}

// Mixin of the CloudInstance.
func (CloudInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

// Indexes of the CloudInstance.
func (CloudInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("cloud_account_id", "region", "instance_id").Unique().StorageKey("uidx_ops_cloud_instance_key"),
		index.Fields("cloud_account_id").StorageKey("idx_ops_cloud_instance_account"),
	}
}
