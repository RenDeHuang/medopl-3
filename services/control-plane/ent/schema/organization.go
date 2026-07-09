package schema

import "entgo.io/ent"

type Organization struct{ ent.Schema }

func (Organization) Fields() []ent.Field { return organizationFields() }
