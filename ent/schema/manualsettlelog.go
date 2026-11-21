package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ManualSettleLog struct{ ent.Schema }

func (ManualSettleLog) Annotations() []schema.Annotation {
	return tableAnno("manual_settle_logs", "人工补单审计日志")
}

func (ManualSettleLog) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("order_id").Comment("订单 ID"),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.String("actor_type").MaxLen(16).Comment("操作者类型：console/admin"),
		field.Int64("actor_id").Comment("操作者 ID"),
		field.String("action").MaxLen(32).Comment("动作：bind_orphan/manual_pay"),
		field.Int64("orphan_id").Optional().Nillable().Comment("关联孤儿单 ID"),
		field.String("remark").MaxLen(512).Default("").Comment("备注"),
		field.String("before_status").MaxLen(16).Default("").Comment("操作前订单状态"),
		field.String("after_status").MaxLen(16).Default("").Comment("操作后订单状态"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (ManualSettleLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id"),
		index.Fields("user_id", "created_at"),
	}
}
