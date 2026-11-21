package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AmountLock struct{ ent.Schema }

func (AmountLock) Annotations() []schema.Annotation {
	return tableAnno("amount_locks", "金额槽位锁（微调占额）")
}

func (AmountLock) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.String("channel").MaxLen(16).Comment("渠道：wechat/alipay"),
		field.Int("pay_amount_cents").Comment("锁定的实付金额（分）"),
		field.Int64("order_id").Comment("关联订单 ID"),
		field.Time("locked_until").Comment("锁到期时间"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (AmountLock) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "channel", "pay_amount_cents"),
		index.Fields("order_id").Unique(),
		index.Fields("locked_until"),
	}
}
