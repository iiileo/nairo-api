package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PlatformSetting 平台级键值配置（如收款商户）。
type PlatformSetting struct{ ent.Schema }

func (PlatformSetting) Annotations() []schema.Annotation {
	return tableAnno("platform_settings", "平台配置")
}

func (PlatformSetting) Fields() []ent.Field {
	return []ent.Field{
		snowflakeID(),
		field.String("key").MaxLen(64).NotEmpty().Unique().Comment("配置键"),
		field.Text("value").Default("").Comment("配置值（可存 JSON）"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
	}
}

func (PlatformSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
	}
}
