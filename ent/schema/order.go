package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Order struct{ ent.Schema }

func (Order) Annotations() []schema.Annotation {
	return tableAnno("orders", "支付订单")
}

func (Order) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("收款商户用户 ID"),
		field.String("out_trade_no").MaxLen(64).NotEmpty().Comment("商户侧订单号"),
		field.String("purpose").MaxLen(16).Default("collect").Comment("用途：collect 收款 / recharge 手续费充值 / test 在线测试"),
		field.Int64("payer_user_id").Optional().Nillable().Comment("付款/充值受益商户 ID（recharge/test）"),
		field.Int("amount_cents").Comment("标价金额（分）"),
		field.Int("pay_amount_cents").Comment("实付金额（分，含微调）"),
		field.Int("adjust_offset_cents").Default(0).Comment("微调偏移（分）"),
		field.String("channel").MaxLen(16).Comment("支付渠道：wechat/alipay"),
		field.String("status").MaxLen(16).Default("pending").Comment("状态：pending/paid/expired/closed"),
		field.String("notify_url").MaxLen(512).Default("").Comment("商户异步通知 URL"),
		field.String("return_url").MaxLen(512).Default("").Comment("支付成功后收银台跳转 URL"),
		field.Int64("pay_code_id").Optional().Nillable().Comment("分配的收款码 ID"),
		field.Int64("account_id").Optional().Nillable().Comment("收款支付账户 ID"),
		field.String("settle_type").MaxLen(16).Default("auto").Comment("入账方式：auto/manual"),
		field.String("manual_remark").MaxLen(512).Optional().Nillable().Comment("人工补单备注"),
		field.String("manual_by_type").MaxLen(16).Optional().Nillable().Comment("补单操作者类型：console/admin"),
		field.Int64("manual_by_id").Optional().Nillable().Comment("补单操作者 ID"),
		field.Time("manual_at").Optional().Nillable().Comment("人工补单时间"),
		field.Time("paid_at").Optional().Nillable().Comment("支付完成时间"),
		field.Time("expire_at").Optional().Nillable().Comment("订单过期时间"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (Order) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "out_trade_no").Unique(),
		index.Fields("user_id", "status"),
		index.Fields("channel", "pay_amount_cents", "status"),
		index.Fields("purpose", "status"),
		index.Fields("payer_user_id", "status"),
	}
}
