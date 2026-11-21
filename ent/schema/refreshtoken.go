package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RefreshToken struct{ ent.Schema }

func (RefreshToken) Annotations() []schema.Annotation {
	return tableAnno("refresh_tokens", "Console 刷新令牌")
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.String("token_hash").MaxLen(128).Unique().NotEmpty().Comment("刷新令牌哈希"),
		field.Time("expires_at").Comment("过期时间"),
		field.Time("revoked_at").Optional().Nillable().Comment("吊销时间"),
		field.String("user_agent").MaxLen(255).Default("").Comment("客户端 UA"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (RefreshToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("expires_at"),
	}
}
