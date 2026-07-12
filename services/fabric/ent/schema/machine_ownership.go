package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type MachineOwnership struct{ ent.Schema }

func (MachineOwnership) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("resource_id").NotEmpty().Unique(),
		field.String("account_id").NotEmpty(),
		field.String("workspace_id").Default(""),
		field.String("package_id").NotEmpty(),
		field.String("node_pool_id").NotEmpty(),
		field.String("machine_id").NotEmpty().Unique(),
		field.String("instance_id").Optional().Unique(),
		field.String("node_name").Default(""),
		field.String("status").NotEmpty(),
		field.String("provider_request_id").Default(""),
		field.Time("claimed_at"),
		field.Time("released_at").Optional().Nillable(),
	}
}
