package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

func TestOperatorSummaryDerivesOperationalNotifications(t *testing.T) {
	app := newControlPlaneApp()
	for _, row := range []map[string]any{
		{"id": "compute-review", "accountId": "acct-review", "billingStatus": "manual_review"},
		{"id": "compute-receipt", "accountId": "acct-receipt", "billingStatus": "active", "lastBillingError": "ledger_receipt_pending"},
		{"id": "compute-past-due", "accountId": "acct-past-due", "billingStatus": "past_due"},
	} {
		mustStore(t, app.tables.SaveCompute(context.Background(), row))
	}
	for _, row := range []map[string]any{
		{"id": "storage-cleanup", "accountId": "acct-cleanup", "billingStatus": "failed", "lastBillingError": "fabric_prepare_cleanup_failed"},
		{"id": "storage-expiry", "accountId": "acct-expiry", "billingStatus": "past_due", "lastBillingError": "fabric_expiry_destroy_failed"},
	} {
		mustStore(t, app.tables.SaveStorage(context.Background(), row))
	}

	notifications := app.operatorSummary()["notifications"].(map[string]any)
	if notifications["total"] != 6 || notifications["error"] != 3 || notifications["warning"] != 3 {
		t.Fatalf("unexpected notification counts: %#v", notifications)
	}
	codes := map[string]int{}
	for _, item := range notifications["recent"].([]any) {
		notification := item.(map[string]any)
		codes[stringValue(notification["code"])]++
	}
	for code, want := range map[string]int{"manual_review": 1, "past_due": 2, "ledger_receipt_pending": 1, "cleanup_failed": 2} {
		if codes[code] != want {
			t.Fatalf("notification code %q count = %d, want %d: %#v", code, codes[code], want, notifications)
		}
	}
	payload, err := json.Marshal(notifications)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "sub2apiRedeemCode") || strings.Contains(string(payload), "lastBillingError") {
		t.Fatalf("operator notification leaked billing internals: %s", payload)
	}
}

func TestMonthlyOperationalLogsAreStableDeduplicatedAndRedacted(t *testing.T) {
	var output bytes.Buffer
	previousOutput, previousFlags, previousPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	app := newControlPlaneApp()
	row := map[string]any{
		"id":                "compute-sensitive-id",
		"accountId":         "acct-sensitive",
		"billingStatus":     "manual_review",
		"lastBillingError":  "provider-secret-detail",
		"sub2apiRedeemCode": "redeem-sensitive",
	}
	mustStore(t, app.saveMonthlyResource(context.Background(), "compute", row))
	mustStore(t, app.saveMonthlyResource(context.Background(), "compute", row))
	row["billingStatus"] = "active"
	delete(row, "lastBillingError")
	mustStore(t, app.saveMonthlyResource(context.Background(), "compute", row))

	logs := output.String()
	if strings.Count(logs, "event=opl_operational_state code=manual_review state=active") != 1 {
		t.Fatalf("active transition should be logged once: %q", logs)
	}
	if strings.Count(logs, "event=opl_operational_state code=manual_review state=recovered") != 1 {
		t.Fatalf("recovery transition should be logged once: %q", logs)
	}
	for _, secret := range []string{"compute-sensitive-id", "acct-sensitive", "provider-secret-detail", "redeem-sensitive"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("operational log leaked %q: %q", secret, logs)
		}
	}
}
