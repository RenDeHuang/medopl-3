package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Connector struct{ ent.Schema }

func (Connector) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "fabric_connectors"}}
}

func (Connector) Fields() []ent.Field {
	return []ent.Field{
		idField(), field.String("connector_id").NotEmpty(), field.String("version").NotEmpty(),
		field.String("version_identity").NotEmpty().Unique(), field.String("digest").NotEmpty(), field.String("name").NotEmpty(),
		field.String("status").NotEmpty(), field.Bool("read_only").Default(true), field.String("provider").NotEmpty(),
		field.String("resource_metadata").NotEmpty(), field.String("runtime_metadata").NotEmpty(), createdAtField(),
	}
}

func (Connector) Indexes() []ent.Index {
	return []ent.Index{index.Fields("connector_id", "version").Unique(), index.Fields("status")}
}
