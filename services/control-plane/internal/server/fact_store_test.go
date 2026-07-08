package server

import (
	"os"
	"strings"
	"testing"
)

func TestPostgresFactStoreDoesNotCreatePayloadJSONB(t *testing.T) {
	data, err := os.ReadFile("fact_store.go")
	if err != nil {
		t.Fatalf("read fact store: %v", err)
	}
	source := string(data)
	forbidden := "payload " + "JSONB"
	if strings.Contains(source, forbidden) || strings.Contains(source, strings.ToLower(forbidden)) {
		t.Fatalf("control-plane facts must use explicit columns, not %s", forbidden)
	}
}
