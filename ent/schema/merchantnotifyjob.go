package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MerchantNotifyJob struct{ ent.Schema }

func (MerchantNotifyJob) Annotations() []schema.Annotation {
	return tableAnno("merchant_notify_jobs", "商户支付结果回调任务（持久化重试）")
}

func (MerchantNotifyJob) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("商户用户 ID"),
		field.Int64("order_id").Comment("订单 ID"),
		field.String("out_trade_no").MaxLen(64).Comment("商户订单号"),
		field.String("notify_url").MaxLen(512).Comment("回调 URL"),
		field.Text("payload").Comment("POST JSON 正文"),
		field.String("status").MaxLen(16).Default("pending").Comment("pending/done/failed"),
		field.Int("attempts").Default(0).Comment("已尝试次数"),
		field.Int("max_attempts").Default(8).Comment("最大尝试次数"),
		field.Time("next_retry_at").Optional().Nillable().Comment("下次重试时间"),
		field.String("last_error").MaxLen(512).Default("").Comment("最近错误"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (MerchantNotifyJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "next_retry_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("order_id"),
	}
}
