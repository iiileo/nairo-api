package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

var (
	incrementalDisabled = false
	withCommentsEnabled = true
)

// snowflakeID 统一主键：BIGINT，禁用自增，应用层写入雪花 ID。
func snowflakeID() ent.Field {
	return field.Int64("id").
		Immutable().
		Positive().
		Comment("主键（雪花 ID）").
		Annotations(entsql.Annotation{Incremental: &incrementalDisabled})
}

func tableAnno(table, comment string) []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: table},
		entsql.WithComments(withCommentsEnabled),
		schema.Comment(comment),
	}
}
