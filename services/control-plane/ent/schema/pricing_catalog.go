package schema

import "entgo.io/ent"

type PricingCatalog struct{ ent.Schema }

func (PricingCatalog) Fields() []ent.Field { return pricingCatalogFields() }
