package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestResourceAutoRenewIntentInterleavings(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		testResourceAutoRenewIntentInterleavings(t, newMemoryTableStore())
	})
	t.Run("postgres", func(t *testing.T) {
		databaseURL := os.Getenv("CONTROL_PLANE_TEST_DATABASE_URL")
		if databaseURL == "" {
			t.Skip("CONTROL_PLANE_TEST_DATABASE_URL is not set")
		}
		admin, err := sql.Open("postgres", databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		if err := admin.Ping(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = admin.Close() })
		schema := fmt.Sprintf("control_plane_resource_intent_%d", time.Now().UnixNano())
		if _, err := admin.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + pq.QuoteIdentifier(schema) + ` CASCADE`) })
		stateStore, err := NewPostgresEntStateStore(postgresInvitedAccountTestURL(databaseURL, schema))
		if err != nil {
			t.Fatal(err)
		}
		store := stateStore.(*postgresEntStateStore)
		t.Cleanup(func() { _ = store.client.Close() })
		testResourceAutoRenewIntentInterleavings(t, store)
	})
}

func TestMemoryResourceAutoRenewRequiresExplicitSetter(t *testing.T) {
	ctx := context.Background()
	store := newMemoryTableStore()
	for _, resourceType := range []string{"compute", "storage"} {
		accountID := "acct-explicit-intent-" + resourceType
		row := intentResource(resourceType, "resource-explicit-intent-"+resourceType, accountID)
		saveIntentResource(t, ctx, store, resourceType, row)
		stale := cloneMap(row)
		stale["autoRenew"] = false
		saveIntentResource(t, ctx, store, resourceType, stale)
		if current := loadIntentResource(t, ctx, store, resourceType, stringValue(row["id"]), accountID); current["autoRenew"] != true {
			t.Fatalf("ordinary %s save changed intent: %#v", resourceType, current)
		}
		if err := store.SetResourceAutoRenew(ctx, resourceType, stringValue(row["id"]), accountID, false); err != nil {
			t.Fatal(err)
		}
		if current := loadIntentResource(t, ctx, store, resourceType, stringValue(row["id"]), accountID); current["autoRenew"] != false {
			t.Fatalf("explicit %s setter did not change intent: %#v", resourceType, current)
		}
	}
}

func testResourceAutoRenewIntentInterleavings(t *testing.T, store controlPlaneTableStore) {
	t.Helper()
	for index, resourceType := range []string{"compute", "storage"} {
		t.Run(resourceType+"/claim_then_disable", func(t *testing.T) {
			ctx := context.Background()
			accountID := "acct-intent-a-" + resourceType
			user := seedIntentOwner(t, ctx, store, accountID, "usr-intent-a-"+resourceType, int64(201+index*2))
			resource := intentResource(resourceType, "resource-intent-a-"+resourceType, accountID)
			saveIntentResource(t, ctx, store, resourceType, resource)

			claim := cloneMap(resource)
			claim["billingStatus"], claim["billingOperationId"] = "renewal_pending", "renewal-intent-a-"+resourceType
			claimed, fresh, err := store.ClaimResourceBillingOperation(ctx, resourceType, claim)
			if err != nil || !fresh {
				t.Fatalf("initial renewal claim fresh=%v row=%#v err=%v", fresh, claimed, err)
			}
			user["status"] = "disabled"
			if err := store.ApplyUserLifecycle(ctx, user); err != nil {
				t.Fatal(err)
			}

			staleProgress := cloneMap(claimed)
			staleProgress["billingStatus"], staleProgress["paidThrough"], staleProgress["autoRenew"] = "active", "2026-09-01T00:00:00Z", true
			saveIntentResource(t, ctx, store, resourceType, staleProgress)
			current := loadIntentResource(t, ctx, store, resourceType, stringValue(resource["id"]), accountID)
			if current["autoRenew"] != false || current["billingStatus"] != "active" || current["paidThrough"] != "2026-09-01T00:00:00Z" {
				t.Fatalf("stale progress restored intent or lost progress: %#v", current)
			}

			next := cloneMap(staleProgress)
			next["billingStatus"], next["billingOperationId"] = "renewal_pending", "renewal-intent-next-"+resourceType
			replayed, fresh, err := store.ClaimResourceBillingOperation(ctx, resourceType, next)
			if err != nil || fresh || replayed["billingOperationId"] != claimed["billingOperationId"] || replayed["paidThrough"] != current["paidThrough"] {
				t.Fatalf("disabled next renewal fresh=%v row=%#v err=%v", fresh, replayed, err)
			}
		})

		t.Run(resourceType+"/disable_then_claim", func(t *testing.T) {
			ctx := context.Background()
			accountID := "acct-intent-b-" + resourceType
			user := seedIntentOwner(t, ctx, store, accountID, "usr-intent-b-"+resourceType, int64(202+index*2))
			resource := intentResource(resourceType, "resource-intent-b-"+resourceType, accountID)
			saveIntentResource(t, ctx, store, resourceType, resource)
			staleClaim := cloneMap(resource)
			staleClaim["billingStatus"], staleClaim["billingOperationId"] = "renewal_pending", "renewal-intent-b-"+resourceType

			user["status"] = "disabled"
			if err := store.ApplyUserLifecycle(ctx, user); err != nil {
				t.Fatal(err)
			}
			claimed, fresh, err := store.ClaimResourceBillingOperation(ctx, resourceType, staleClaim)
			if err != nil || fresh || claimed["billingOperationId"] != resource["billingOperationId"] || claimed["paidThrough"] != resource["paidThrough"] || claimed["autoRenew"] != false {
				t.Fatalf("claim after disable fresh=%v row=%#v err=%v", fresh, claimed, err)
			}
		})
	}
}

func seedIntentOwner(t *testing.T, ctx context.Context, store controlPlaneTableStore, accountID, userID string, sub2APIUserID int64) map[string]any {
	t.Helper()
	mustStore(t, store.SaveAccount(ctx, map[string]any{"id": accountID, "status": "active", "sub2apiUserId": sub2APIUserID}))
	user := map[string]any{"id": userID, "email": userID + "@example.com", "accountId": accountID, "role": "owner", "status": "active"}
	mustStore(t, store.SaveUser(ctx, user))
	return user
}

func intentResource(resourceType, id, accountID string) map[string]any {
	return map[string]any{
		"id": id, "resourceType": resourceType, "accountId": accountID, "autoRenew": true,
		"billingStatus": "active", "billingOperationId": "purchase-" + id,
		"pricingVersion": "2026-07-14", "packageId": "basic", "periodStart": "2026-07-01T00:00:00Z", "paidThrough": "2026-08-01T00:00:00Z",
		"monthlyPriceCnyCents": int64(35000), "chargeUsdMicros": int64(50_000_000),
	}
}

func saveIntentResource(t *testing.T, ctx context.Context, store controlPlaneTableStore, resourceType string, row map[string]any) {
	t.Helper()
	var err error
	if resourceType == "storage" {
		err = store.SaveStorage(ctx, row)
	} else {
		err = store.SaveCompute(ctx, row)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func loadIntentResource(t *testing.T, ctx context.Context, store controlPlaneTableStore, resourceType, id, accountID string) map[string]any {
	t.Helper()
	var rows []map[string]any
	var err error
	if resourceType == "storage" {
		rows, err = store.ListStorages(ctx, accountID)
	} else {
		rows, err = store.ListComputes(ctx, accountID)
	}
	if err != nil {
		t.Fatal(err)
	}
	row := findRecord(rows, id)
	if row == nil {
		t.Fatalf("%s %s not found in %#v", resourceType, id, rows)
	}
	return row
}
