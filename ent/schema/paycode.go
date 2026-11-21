package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PayCode struct{ ent.Schema }

func (PayCode) Annotations() []schema.Annotation {
	return tableAnno("pay_codes", "收款码")
}

func (PayCode) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("account_id").Comment("所属支付账户 ID"),
		field.String("channel").MaxLen(16).Default("").Comment("收款码渠道：wechat/alipay"),
		field.String("code_type").MaxLen(16).Default("float").Comment("类型：float 任意金额 / fixed 固额"),
		field.Int("amount_cents").Optional().Nillable().Comment("固额金额（分），float 时可空"),
		field.String("image_url").MaxLen(512).Default("").Comment("收款码图片 URL（可选；优先存 payload）"),
		field.Text("payload").Optional().Nillable().Comment("二维码解码内容（微信/支付宝收款链接）"),
		field.String("status").MaxLen(16).Default("active").Comment("状态：active/disabled"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (PayCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id"),
		index.Fields("account_id", "channel", "code_type", "status"),
	}
}
