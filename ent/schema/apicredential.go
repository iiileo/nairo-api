package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ApiCredential struct{ ent.Schema }

func (ApiCredential) Annotations() []schema.Annotation {
	return tableAnno("api_credentials", "商户 Open API 凭证")
}

func (ApiCredential) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.Int64("user_id").Comment("所属商户用户 ID"),
		field.String("access_key").MaxLen(64).Unique().NotEmpty().Comment("Access Key（公开标识）"),
		field.String("secret_hash").MaxLen(255).NotEmpty().Comment("API Secret（HMAC 签名用，MVP 存明文）"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

func (ApiCredential) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("access_key").Unique(),
	}
}
