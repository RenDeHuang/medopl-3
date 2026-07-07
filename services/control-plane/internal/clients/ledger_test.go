package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLedgerHTTPClientReturnsErrorForFailedMutation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ledger/holds/release" {
			t.Fatalf("path = %s, want /ledger/holds/release", r.URL.Path)
		}
		http.Error(w, "insufficient frozen balance", http.StatusConflict)
	}))
	defer server.Close()

	client := NewLedgerHTTPClient(server.URL, server.Client())
	_, err := client.ReleaseHold(context.Background(), HoldReleaseInput{AccountID: "acct-alpha", AmountCents: 1000, Currency: "CNY"}, "release-once")
	if err == nil || !strings.Contains(err.Error(), "status 409") {
		t.Fatalf("expected status error, got %v", err)
	}
}
