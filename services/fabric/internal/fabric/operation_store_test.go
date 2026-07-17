package fabric

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"opl-cloud/services/fabric/ent/contenttransfer"
	"opl-cloud/services/fabric/ent/contenttransferchunk"
)

func TestMemoryOperationStoreReclaimRuntimeFencesOldOwner(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryOperationStore()
	oldStartedAt := time.Date(2026, 7, 17, 0, 0, 0, 123456000, time.UTC)
	operation := newOperation("create_workspace_runtime", "workspace_runtime", "workspace-alpha", "acct-alpha", "workspace-alpha", "runtime-fence", "request-hash", oldStartedAt)
	operation.ID = "fop-runtime-fence"
	operation.Status = "started"
	operation.ErrorCode = "stale_error"
	operation.FinishedAt = oldStartedAt.Add(time.Second)
	operation.CreatedAt = oldStartedAt
	oldOwner, claimed, err := store.ClaimRuntime(ctx, operation)
	if err != nil || !claimed {
		t.Fatalf("claim old owner=%#v claimed=%v err=%v", oldOwner, claimed, err)
	}

	newStartedAt := oldStartedAt.Add(3 * time.Minute)
	newOwner, won, err := store.ReclaimRuntime(ctx, oldOwner.ID, oldOwner.StartedAt, newStartedAt)
	if err != nil || !won || !newOwner.StartedAt.Equal(newStartedAt) || !newOwner.FinishedAt.IsZero() || newOwner.ErrorCode != "" {
		t.Fatalf("reclaim new owner=%#v won=%v err=%v", newOwner, won, err)
	}
	current, won, err := store.ReclaimRuntime(ctx, oldOwner.ID, oldOwner.StartedAt, newStartedAt.Add(time.Second))
	if err != nil || won || !current.StartedAt.Equal(newStartedAt) {
		t.Fatalf("losing reclaim current=%#v won=%v err=%v", current, won, err)
	}

	oldOwner.Status = "succeeded"
	oldOwner.FinishedAt = newStartedAt.Add(time.Second)
	oldOwner.RedactedProviderPayload = map[string]any{"resource": WorkspaceRuntime{ID: "runtime-old", WorkspaceID: "workspace-alpha"}}
	if err := store.SaveRuntime(ctx, oldOwner); !errors.Is(err, ErrRuntimeOperationNotCurrent) {
		t.Fatalf("old owner save error=%v, want ErrRuntimeOperationNotCurrent", err)
	}
	newOwner.Status = "succeeded"
	newOwner.FinishedAt = newStartedAt.Add(2 * time.Second)
	newOwner.RedactedProviderPayload = map[string]any{"resource": WorkspaceRuntime{ID: "runtime-current", WorkspaceID: "workspace-alpha"}}
	if err := store.SaveRuntime(ctx, newOwner); err != nil {
		t.Fatalf("new owner save: %v", err)
	}
	operations, err := store.List(ctx)
	if err != nil || len(operations) != 1 || operations[0].Status != "succeeded" || !operations[0].StartedAt.Equal(newStartedAt) {
		t.Fatalf("final operations=%#v err=%v", operations, err)
	}
	var runtime WorkspaceRuntime
	if !decodeOperationResource(operations[0], &runtime) || runtime.ID != "runtime-current" {
		t.Fatalf("old owner overwrote current result: runtime=%#v operation=%#v", runtime, operations[0])
	}
}

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
		"CREATE UNIQUE INDEX IF NOT EXISTS fabric_operations_runtime_claim_idx",
	} {
		if !strings.Contains(schema, marker) {
			t.Fatalf("schema missing %q", marker)
		}
	}
	if strings.Contains(schema, "JSONB") {
		t.Fatalf("fabric schema must not keep JSONB fact columns")
	}
}

func TestRuntimeClaimMigrationMatchesEmbeddedCopy(t *testing.T) {
	formal, err := os.ReadFile("../../migrations/202607110003_runtime_operation_claim.sql")
	if err != nil {
		t.Fatalf("read formal migration: %v", err)
	}
	embedded, err := os.ReadFile("ent_migrations/202607110003_runtime_operation_claim.sql")
	if err != nil {
		t.Fatalf("read embedded migration: %v", err)
	}
	if !bytes.Equal(formal, embedded) {
		t.Fatal("formal and embedded runtime claim migrations differ")
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
