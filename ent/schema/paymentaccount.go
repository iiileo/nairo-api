package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PaymentAccount struct{ ent.Schema }

func (PaymentAccount) Annotations() []schema.Annotation {
	return tableAnno("payment_accounts", "支付账户（微信/支付宝收款号）")
}

func (PaymentAccount) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("所属商户用户 ID"),
		field.String("channel").MaxLen(64).Comment("支持渠道（逗号分隔）：wechat/alipay，可同时支持"),
		field.String("name").MaxLen(64).Default("").Comment("账户备注名"),
		field.String("callback_id").MaxLen(32).Unique().NotEmpty().Comment("App 回调短码"),
		field.String("app_secret_hash").MaxLen(255).NotEmpty().Comment("App 推送签名密钥"),
		field.String("status").MaxLen(16).Default("active").Comment("状态：active/disabled"),
		field.Time("app_last_seen_at").Optional().Nillable().Comment("App 最近在线时间"),
		field.String("bound_device_id").MaxLen(64).Default("").Comment("已绑定手机的安装 ID（空=未绑定；一个监控设备同时只能被一部手机绑定）"),
		field.String("bound_device_name").MaxLen(64).Default("").Comment("已绑定手机型号（展示用）"),
		field.Time("bound_at").Optional().Nillable().Comment("手机绑定时间"),
		field.String("app_conn_type").MaxLen(16).Optional().Nillable().Comment("连接类型：http/ws"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (PaymentAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("callback_id").Unique(),
		index.Fields("user_id", "channel"),
	}
}
