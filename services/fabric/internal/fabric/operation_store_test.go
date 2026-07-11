package fabric

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"opl-cloud/services/fabric/ent/contenttransfer"
	"opl-cloud/services/fabric/ent/contenttransferchunk"
)

func TestPostgresOperationSchemaDefinesFabricOperationsAuditTable(t *testing.T) {
	schema := PostgresOperationSchemaSQL()
	for _, marker := range []string{
		"CREATE TABLE IF NOT EXISTS fabric_operations",
		"operation_id TEXT NOT NULL",
		"caller_service TEXT NOT NULL",
		"resource_kind TEXT NOT NULL",
		"provider_request_id TEXT NOT NULL DEFAULT ''",
		"request_hash TEXT NOT NULL DEFAULT ''",
		"redacted_provider_payload TEXT NOT NULL DEFAULT '{}'",
		"CREATE INDEX IF NOT EXISTS fabric_operations_resource_idx",
	} {
		if !strings.Contains(schema, marker) {
			t.Fatalf("schema missing %q", marker)
		}
	}
	if strings.Contains(schema, "JSONB") {
		t.Fatalf("fabric schema must not keep JSONB fact columns")
	}
}

func TestPostgresOperationSchemaDefinesContentTransferTables(t *testing.T) {
	schema := PostgresOperationSchemaSQL()
	for _, table := range []string{contenttransfer.Table, contenttransferchunk.Table} {
		if !strings.Contains(schema, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("schema missing content transfer table %q", table)
		}
	}
}

func TestPostgresOperationSchemaDropsRetiredWorkspaceRuntimeAccessTable(t *testing.T) {
	schema := PostgresOperationSchemaSQL()
	createAt := strings.Index(schema, "CREATE TABLE IF NOT EXISTS fabric_workspace_runtime_access")
	dropAt := strings.Index(schema, "DROP TABLE IF EXISTS fabric_workspace_runtime_access")
	if dropAt < 0 || dropAt < createAt {
		t.Fatal("Fabric hard-cut migration must drop the retired runtime access table")
	}
}

func TestPostgresOperationSchemaDefinesImmutableCatalogTables(t *testing.T) {
	schema := PostgresOperationSchemaSQL()
	for _, marker := range []string{
		"CREATE TABLE IF NOT EXISTS fabric_connectors",
		"CREATE TABLE IF NOT EXISTS fabric_environment_templates",
		"UNIQUE (connector_id, version)",
		"UNIQUE (template_id, version)",
		"resource_metadata TEXT NOT NULL",
		"runtime_metadata TEXT NOT NULL",
		"fabric_connector_identity_immutable",
		"fabric_environment_template_identity_immutable",
	} {
		if !strings.Contains(schema, marker) {
			t.Fatalf("catalog schema missing %q", marker)
		}
	}
	formal, err := os.ReadFile("../../migrations/202607110002_fabric_catalog.sql")
	if err != nil {
		t.Fatal(err)
	}
	local, err := os.ReadFile("ent_migrations/202607110002_fabric_catalog.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(formal, local) {
		t.Fatal("formal and embedded catalog migrations differ")
	}
}

func TestPostgresMigrationChainRejectsStandalonePatchMarkers(t *testing.T) {
	for lineNumber, line := range strings.Split(PostgresOperationSchemaSQL(), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.Trim(trimmed, "+-@*") == "" {
			t.Fatalf("migration chain line %d is a standalone non-SQL patch marker: %q", lineNumber+1, line)
		}
	}
}
