package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type IdempotencyRecord struct{ ent.Schema }

func (IdempotencyRecord) Annotations() []schema.Annotation {
	return tableAnno("idempotency_records", "幂等键（推送/通知去重）")
}

func (IdempotencyRecord) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.String("scope").MaxLen(32).NotEmpty().Comment("作用域：app_push/bot_notify"),
		field.String("idem_key").MaxLen(128).NotEmpty().Comment("幂等键"),
		field.String("result").MaxLen(32).Default("").Comment("处理结果摘要"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (IdempotencyRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope", "idem_key").Unique(),
		index.Fields("created_at"),
	}
}
