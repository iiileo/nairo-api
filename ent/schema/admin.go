package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Admin struct{ ent.Schema }

func (Admin) Annotations() []schema.Annotation {
	return tableAnno("admins", "平台管理员")
}

func (Admin) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.String("username").MaxLen(64).Unique().NotEmpty().Comment("登录用户名"),
		field.String("password_hash").MaxLen(255).NotEmpty().Comment("密码哈希"),
		field.String("name").MaxLen(64).Default("").Comment("显示名称"),
		field.String("status").MaxLen(16).Default("active").Comment("状态：active/disabled"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (Admin) Indexes() []ent.Index {
	return []ent.Index{index.Fields("username").Unique()}
}
