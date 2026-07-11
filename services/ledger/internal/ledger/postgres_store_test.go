package ledger

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func TestPostgresSchemaUsesEntMigrationLedgerTables(t *testing.T) {
	schema := PostgresSchemaSQL()
	required := []string{
		"CREATE TABLE IF NOT EXISTS wallets",
		"ALTER TABLE wallets ADD COLUMN IF NOT EXISTS available_cents",
		"CREATE TABLE IF NOT EXISTS ledger_entries",
		"CREATE TABLE IF NOT EXISTS wallet_transactions",
		"CREATE TABLE IF NOT EXISTS manual_topups",
		"CREATE TABLE IF NOT EXISTS holds",
		"CREATE TABLE IF NOT EXISTS hold_releases",
		"CREATE TABLE IF NOT EXISTS evidence_receipts",
		"organization_id TEXT NOT NULL DEFAULT ''",
		"project_id TEXT NOT NULL DEFAULT ''",
		"task_id TEXT NOT NULL DEFAULT ''",
		"job_id TEXT NOT NULL DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS evidence_receipts_organization_created",
		"CREATE INDEX IF NOT EXISTS evidence_receipts_workspace_created",
		"CREATE TABLE IF NOT EXISTS resource_settlements",
		"price_snapshot_json TEXT NOT NULL DEFAULT '{}'",
		"CREATE TABLE IF NOT EXISTS reconciliation_reports",
		"report_json TEXT NOT NULL DEFAULT '{}'",
		"CREATE TABLE IF NOT EXISTS idempotency_keys",
	}
	for _, marker := range required {
		if !strings.Contains(schema, marker) {
			t.Fatalf("schema missing %q", marker)
		}
	}
	if strings.Contains(schema, "JSONB") {
		t.Fatalf("ledger schema must not keep JSONB fact columns")
	}
	generatedValidators := []string{
		`validator failed for field "Hold.workspace_id"`,
		`validator failed for field "HoldRelease.workspace_id"`,
		`validator failed for field "ResourceSettlement.workspace_id"`,
	}
	for _, marker := range generatedValidators {
		if strings.Contains(readGeneratedLedgerEnt(t), marker) {
			t.Fatalf("ledger resource facts must allow account/resource scoped rows before workspace exists: found %q", marker)
		}
	}
}

func TestPostgresStoreImplementsLedgerStore(t *testing.T) {
	var db *sql.DB
	var _ Store = NewPostgresStore(db)
}

func readGeneratedLedgerEnt(t *testing.T) string {
	t.Helper()
	files := []string{
		"../../ent/hold_create.go",
		"../../ent/holdrelease_create.go",
		"../../ent/resourcesettlement_create.go",
	}
	var out strings.Builder
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		out.Write(data)
	}
	return out.String()
}
