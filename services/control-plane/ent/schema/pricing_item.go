package schema

import "entgo.io/ent"

type PricingItem struct{ ent.Schema }

func (PricingItem) Fields() []ent.Field { return pricingItemFields() }
