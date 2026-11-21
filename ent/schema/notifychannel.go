package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NotifyChannel 商户 IM 群机器人（企业微信 / 飞书 / 钉钉）。
type NotifyChannel struct{ ent.Schema }

func (NotifyChannel) Annotations() []schema.Annotation {
	return tableAnno("notify_channels", "商户通知机器人（企业微信/飞书/钉钉）")
}

func (NotifyChannel) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("所属商户用户 ID"),
		field.Int64("account_id").Optional().Nillable().Comment("绑定监控设备；空=商户级（全部设备）"),
		field.String("provider").MaxLen(16).NotEmpty().Comment("渠道：wecom/feishu/dingtalk"),
		field.String("name").MaxLen(64).Default("").Comment("备注名"),
		field.String("webhook_url").MaxLen(512).NotEmpty().Comment("Webhook URL"),
		field.String("secret").MaxLen(255).Optional().Nillable().Comment("加签密钥（钉钉/飞书可选）"),
		field.Bool("enabled").Default(true).Comment("是否启用"),
		field.Bool("event_order_paid").Default(true).Comment("订单入账通知"),
		field.Bool("event_fee_low").Default(true).Comment("手续费不足通知"),
		field.Bool("event_app_push").Default(true).Comment("收到 App 支付推送通知"),
		field.Bool("event_device_offline").Default(true).Comment("监控设备心跳丢失告警"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (NotifyChannel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_id", "provider"),
		index.Fields("user_id", "enabled"),
		index.Fields("user_id", "account_id"),
	}
}
