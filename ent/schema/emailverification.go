package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type EmailVerification struct{ ent.Schema }

func (EmailVerification) Annotations() []schema.Annotation {
	return tableAnno("email_verifications", "邮箱验证码")
}

func (EmailVerification) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.String("email").MaxLen(128).NotEmpty().Comment("邮箱"),
		field.String("code_hash").MaxLen(128).NotEmpty().Comment("验证码哈希"),
		field.String("purpose").MaxLen(32).Default("register").Comment("用途：register/reset_password"),
		field.Time("expires_at").Comment("过期时间"),
		field.Time("used_at").Optional().Nillable().Comment("使用时间"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (EmailVerification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email", "purpose"),
		index.Fields("expires_at"),
	}
}
