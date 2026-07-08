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

func TestPostgresFactStoreRepairsOldFactTablesBeforeLoading(t *testing.T) {
	data, err := os.ReadFile("fact_store.go")
	if err != nil {
		t.Fatalf("read fact store: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS `+timestampColumn+` TIMESTAMPTZ NOT NULL DEFAULT now()",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("postgres fact install must repair old tables with %q", required)
		}
	}
}
