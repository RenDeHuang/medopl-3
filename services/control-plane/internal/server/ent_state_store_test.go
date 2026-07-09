package server

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"

	controlplaneenttest "opl-cloud/services/control-plane/ent/enttest"
	"opl-cloud/services/control-plane/ent/pricingitem"
)

func NewTestEntStateStore(t *testing.T, path string) StateStore {
	t.Helper()
	client := controlplaneenttest.Open(t, dialect.SQLite, path+"?_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	return &postgresEntStateStore{client: client}
}

func TestEntStateStorePricingCatalogReadsPricingTables(t *testing.T) {
	ctx := context.Background()
	store := NewTestEntStateStore(t, t.TempDir()+"/pricing.sqlite").(*postgresEntStateStore)

	if _, err := store.PricingCatalog(ctx); err != nil {
		t.Fatalf("seed pricing catalog: %v", err)
	}
	if _, err := store.client.PricingItem.Update().
		Where(
			pricingitem.CatalogVersion(pricingCatalogVersion),
			pricingitem.PackageID("basic"),
			pricingitem.ResourceType("compute"),
		).
		SetUnitPrice(2.5).
		SetUnitPriceCents(250).
		Save(ctx); err != nil {
		t.Fatalf("update pricing item: %v", err)
	}

	catalog, err := store.PricingCatalog(ctx)
	if err != nil {
		t.Fatalf("read pricing catalog: %v", err)
	}
	basic := packageByIDFromCatalog(catalog, "basic")
	if basic.ComputeHourly != 2.5 {
		t.Fatalf("pricing catalog must read DB item price, got %#v", basic)
	}
}

func TestEntStateStoreIgnoresDuplicateEventProjectionIDs(t *testing.T) {
	store := NewTestEntStateStore(t, t.TempDir()+"/duplicate-events.sqlite")
	facts := controlPlaneState{
		Ledger: []controlPlaneRecord{
			{"id": "ledger-alpha", "accountId": "acct-alpha", "type": "compute_debit", "amountCents": int64(-100)},
			{"id": "ledger-alpha", "accountId": "acct-alpha", "type": "compute_debit", "amountCents": int64(-100)},
		},
	}
	if err := store.Save(context.Background(), facts); err != nil {
		t.Fatalf("duplicate event projections should not break state persistence: %v", err)
	}
}
