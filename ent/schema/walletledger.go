package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WalletLedger struct{ ent.Schema }

func (WalletLedger) Annotations() []schema.Annotation {
	return tableAnno("wallet_ledgers", "钱包流水")
}

func (WalletLedger) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.String("type").MaxLen(32).Comment("类型：recharge/fee_deduct/adjust"),
		field.Int64("amount_cents").Comment("变动金额（分，可正可负）"),
		field.Int64("balance_after_cents").Comment("变动后余额（分）"),
		field.Int64("order_id").Optional().Nillable().Comment("关联订单 ID"),
		field.Int64("admin_id").Optional().Nillable().Comment("操作管理员 ID（充值时）"),
		field.String("remark").MaxLen(512).Default("").Comment("备注"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (WalletLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("order_id"),
	}
}
