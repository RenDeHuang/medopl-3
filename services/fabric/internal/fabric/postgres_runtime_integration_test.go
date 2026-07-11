package fabric

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPostgresRuntimeClaimAcrossServiceInstances(t *testing.T) {
	databaseURL := os.Getenv("FABRIC_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("FABRIC_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("random schema suffix: %v", err)
	}
	schema := "fabric_test_" + hex.EncodeToString(suffix)
	admin, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open postgres admin: %v", err)
	}
	defer admin.Close()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer admin.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")

	testURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	query := testURL.Query()
	query.Set("search_path", schema)
	query.Set("connect_timeout", "5")
	query.Set("statement_timeout", "10000")
	testURL.RawQuery = query.Encode()

	firstStore, err := NewPostgresOperationStore(testURL.String())
	if err != nil {
		t.Fatalf("open first operation store: %v", err)
	}
	defer firstStore.client.Close()
	secondStore, err := NewPostgresOperationStore(testURL.String())
	if err != nil {
		t.Fatalf("open second operation store: %v", err)
	}
	defer secondStore.client.Close()

	provider := &blockingRuntimeProvider{entered: make(chan struct{}), release: make(chan struct{})}
	firstService := runtimeTestService(provider, firstStore)
	secondService := runtimeTestService(provider, secondStore)
	input := runtimeTestInput("postgres-runtime-shared")
	firstDone := make(chan error, 1)
	go func() {
		_, err := firstService.CreateWorkspaceRuntime(ctx, input)
		firstDone <- err
	}()
	select {
	case <-provider.entered:
	case <-ctx.Done():
		t.Fatal("first provider call did not start")
	}
	if _, err := secondService.CreateWorkspaceRuntime(ctx, input); err != ErrRuntimeOperationInProgress {
		t.Fatalf("concurrent replay error = %v, want %v", err, ErrRuntimeOperationInProgress)
	}
	if calls := provider.calls.Load(); calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}
	close(provider.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first runtime create: %v", err)
	}

	replayed, err := NewServiceWithOperationStore(provider, secondStore).CreateWorkspaceRuntime(ctx, input)
	if err != nil || replayed.ID != "runtime-alpha" || provider.calls.Load() != 1 {
		t.Fatalf("postgres restart replay = %s err=%v providerCalls=%d", fmt.Sprintf("%#v", replayed), err, provider.calls.Load())
	}
}
