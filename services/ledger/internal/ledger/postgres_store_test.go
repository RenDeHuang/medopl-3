package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
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
		"CREATE TABLE IF NOT EXISTS review_policies",
		"required_reviewers_json TEXT NOT NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS review_policies_active_scope",
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

func TestFormalMigrationsDeclareReceiptColumnsBeforeQueryBackfill(t *testing.T) {
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatalf("read formal migrations: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	var migrations strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("../../migrations", name))
		if err != nil {
			t.Fatalf("read formal migration %s: %v", name, err)
		}
		migrations.Write(data)
		migrations.WriteByte('\n')
	}
	sql := migrations.String()
	backfill := strings.Index(sql, "UPDATE evidence_receipts")
	if backfill < 0 {
		t.Fatal("formal migrations missing receipt identity backfill")
	}
	for _, column := range []string{"receipt_type", "status", "payload_json", "supersedes_receipt_id", "organization_id", "project_id", "task_id", "job_id"} {
		declaration := strings.Index(sql, "ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS "+column)
		if declaration < 0 || declaration > backfill {
			t.Fatalf("formal migrations must declare %s before receipt backfill", column)
		}
	}
}

func TestPostgresStoreImplementsLedgerStore(t *testing.T) {
	var db *sql.DB
	var _ Store = NewPostgresStore(db)
}

func TestPostgresConcurrentReviewPolicyIdempotency(t *testing.T) {
	db := openLedgerTestPostgres(t)
	store := NewPostgresStore(db)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := store.Install(ctx); err != nil {
		t.Fatalf("install ledger schema: %v", err)
	}
	input := ReviewPolicyInput{
		ExecutionIdentity: ExecutionIdentity{OrganizationID: "org-concurrent", WorkspaceID: "workspace-concurrent", ProjectID: "project-concurrent", TaskID: "task-concurrent", JobID: "job-concurrent"},
		Version:           "1", RequiredReviewers: []RequiredReviewer{{ReviewerRef: "reviewer-rca", ReviewerVersion: "1.0.0"}}, IdempotencyKey: "policy-concurrent",
	}
	type outcome struct {
		policy ReviewPolicy
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			policy, err := store.CreateReviewPolicy(ctx, input)
			outcomes <- outcome{policy: policy, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)
	results := make([]ReviewPolicy, 0, 2)
	for result := range outcomes {
		if result.err != nil {
			t.Fatalf("concurrent create: %v", result.err)
		}
		results = append(results, result.policy)
	}
	if results[0].PolicyID != results[1].PolicyID || results[0].Replayed == results[1].Replayed {
		t.Fatalf("concurrent results must contain one create and one replay: %#v", results)
	}

	conflict := input
	conflict.Version = "2"
	if _, err := store.CreateReviewPolicy(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different fingerprint error = %v, want ErrIdempotencyConflict", err)
	}
}

func openLedgerTestPostgres(t *testing.T) *sql.DB {
	t.Helper()
	rawURL := os.Getenv("LEDGER_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("LEDGER_TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("postgres", rawURL)
	if err != nil {
		t.Fatalf("open test postgres: %v", err)
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := admin.PingContext(connectCtx); err != nil {
		_ = admin.Close()
		t.Fatalf("connect test postgres: %v", err)
	}
	schema := fmt.Sprintf("ledger_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(connectCtx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	testURL := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("search_path", schema)
		query.Set("connect_timeout", "10")
		parsed.RawQuery = query.Encode()
		testURL = parsed.String()
	} else {
		testURL += " search_path=" + pq.QuoteLiteral(schema) + " connect_timeout=10"
	}
	db, err := sql.Open("postgres", testURL)
	if err != nil {
		_, _ = admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE")
		_ = admin.Close()
		t.Fatalf("open isolated test schema: %v", err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() {
		_ = db.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(cleanupCtx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE")
		_ = admin.Close()
	})
	return db
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
