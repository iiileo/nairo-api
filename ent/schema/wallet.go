package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Wallet struct{ ent.Schema }

func (Wallet) Annotations() []schema.Annotation {
	return tableAnno("wallets", "商户手续费钱包")
}

func (Wallet) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.Int64("balance_cents").Default(0).Comment("余额（分）"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (Wallet) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id").Unique()}
}
