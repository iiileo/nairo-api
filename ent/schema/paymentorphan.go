package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PaymentOrphan struct{ ent.Schema }

func (PaymentOrphan) Annotations() []schema.Annotation {
	return tableAnno("payment_orphans", "未匹配到账（孤儿收款）")
}

func (PaymentOrphan) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.Int64("account_id").Comment("上报的支付账户 ID"),
		field.String("channel").MaxLen(16).Comment("渠道：wechat/alipay"),
		field.Int("pay_amount_cents").Comment("上报金额（分）"),
		field.Time("reported_at").Comment("App 上报时间"),
		field.Text("raw_payload").Optional().Comment("原始上报载荷"),
		field.String("status").MaxLen(16).Default("open").Comment("状态：open/bound/ignored"),
		field.Int64("bound_order_id").Optional().Nillable().Comment("绑定的订单 ID"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (PaymentOrphan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status"),
		index.Fields("account_id", "reported_at"),
	}
}
