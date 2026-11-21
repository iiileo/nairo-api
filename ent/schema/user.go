package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct{ ent.Schema }

func (User) Annotations() []schema.Annotation {
	return tableAnno("users", "商户用户")
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.String("email").MaxLen(128).Optional().Nillable().Comment("邮箱（可登录）"),
		field.String("phone").MaxLen(32).Optional().Nillable().Comment("手机号（可登录）"),
		field.String("password_hash").MaxLen(255).NotEmpty().Comment("密码哈希"),
		field.String("status").MaxLen(16).Default("active").Comment("状态：active/disabled/pending"),
		field.Int("fee_rate_bps").Default(100).Comment("手续费率（基点，100=1%）"),
		field.Bool("adjust_enabled").Default(true).Comment("是否启用金额微调"),
		field.String("adjust_direction").MaxLen(8).Default("inc").Comment("微调方向：inc 递增 / dec 递减"),
		field.Int("adjust_step_cents").Default(1).Comment("微调步长（分）"),
		field.Int("adjust_max_offset_cents").Default(50).Comment("微调最大偏移（分）"),
		field.Time("console_last_seen_at").Optional().Nillable().Comment("控制台最近心跳时间"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email").Unique(),
		index.Fields("phone"),
		index.Fields("status"),
	}
}
