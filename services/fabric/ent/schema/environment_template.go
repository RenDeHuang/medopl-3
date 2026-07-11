package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type EnvironmentTemplate struct{ ent.Schema }

func (EnvironmentTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "fabric_environment_templates"}}
}

func (EnvironmentTemplate) Fields() []ent.Field {
	return []ent.Field{
		idField(), field.String("template_id").NotEmpty(), field.String("version").NotEmpty(),
		field.String("version_identity").NotEmpty().Unique(), field.String("digest").NotEmpty(), field.String("name").NotEmpty(),
		field.String("status").NotEmpty(), field.String("resource_metadata").NotEmpty(), field.String("runtime_metadata").NotEmpty(), createdAtField(),
	}
}

func (EnvironmentTemplate) Indexes() []ent.Index {
	return []ent.Index{index.Fields("template_id", "version").Unique(), index.Fields("status")}
}
