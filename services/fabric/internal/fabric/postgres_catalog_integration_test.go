package fabric

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/lib/pq"

	fabricent "opl-cloud/services/fabric/ent"
)

const catalogInsertBarrierKey = 72411002

func TestPostgresCatalogMigrationsImmutabilityAndConcurrentSeed(t *testing.T) {
	store, db := openFabricTestPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := store.Install(ctx); err != nil {
		t.Fatal("install Fabric migrations failed")
	}
	connectors, templates := defaultCatalogRecords()
	connectorRecord, templateRecord := connectors[0], templates[0]
	if err := store.SeedCatalog(ctx, []Connector{connectorRecord}, []EnvironmentTemplate{templateRecord}); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if err := store.SeedCatalog(ctx, []Connector{connectorRecord}, []EnvironmentTemplate{templateRecord}); err != nil {
		t.Fatalf("matching seed must replay: %v", err)
	}
	changedConnector := connectorRecord
	changedConnector.Name = "Changed PubMed"
	if err := store.SeedCatalog(ctx, []Connector{changedConnector}, nil); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("changed connector error = %v, want ErrCatalogVersionConflict", err)
	}
	changedTemplate := templateRecord
	changedTemplate.Runtime.Image = "changed-image"
	if err := store.SeedCatalog(ctx, nil, []EnvironmentTemplate{changedTemplate}); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("changed template error = %v, want ErrCatalogVersionConflict", err)
	}
	assertImmutableUpdate(t, ctx, db, `UPDATE fabric_connectors SET name = 'forbidden' WHERE connector_id = $1 AND version = $2`, connectorRecord.ID, connectorRecord.Version)
	assertImmutableUpdate(t, ctx, db, `UPDATE fabric_environment_templates SET status = 'disabled' WHERE template_id = $1 AND version = $2`, templateRecord.ID, templateRecord.Version)

	installCatalogInsertBarrier(t, ctx, db)
	matching := catalogConnectorFixture("concurrent-match", "Matching")
	matchingErrors := runConcurrentConnectorSeeds(t, ctx, db, store, matching, matching)
	if matchingErrors[0] != nil || matchingErrors[1] != nil {
		t.Fatalf("matching concurrent seeds = %v", matchingErrors)
	}
	assertConnectorCount(t, ctx, db, matching.ID, matching.Version, 1)

	mismatchLeft := catalogConnectorFixture("concurrent-mismatch", "Left")
	mismatchRight := mismatchLeft
	mismatchRight.Name = "Right"
	mismatchErrors := runConcurrentConnectorSeeds(t, ctx, db, store, mismatchLeft, mismatchRight)
	conflicts := 0
	succeeded := 0
	for _, err := range mismatchErrors {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrCatalogVersionConflict):
			conflicts++
		default:
			t.Fatalf("mismatching concurrent seed error = %v", err)
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("mismatching concurrent seeds = %v", mismatchErrors)
	}
	assertConnectorCount(t, ctx, db, mismatchLeft.ID, mismatchLeft.Version, 1)
}

func openFabricTestPostgres(t *testing.T) (*PostgresOperationStore, *sql.DB) {
	t.Helper()
	rawURL := os.Getenv("FABRIC_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("FABRIC_TEST_DATABASE_URL is not set")
	}
	schema := "fabric_test_" + randomHex(t, 8)
	admin, err := sql.Open("postgres", boundedPostgresURL(rawURL, ""))
	if err != nil {
		t.Fatal("open Fabric test PostgreSQL failed")
	}
	admin.SetMaxOpenConns(2)
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := admin.PingContext(connectCtx); err != nil {
		_ = admin.Close()
		t.Fatal("connect Fabric test PostgreSQL failed")
	}
	if _, err := admin.ExecContext(connectCtx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		_ = admin.Close()
		t.Fatal("create Fabric test schema failed")
	}
	db, err := sql.Open("postgres", boundedPostgresURL(rawURL, schema))
	if err != nil {
		dropTestSchema(admin, schema)
		_ = admin.Close()
		t.Fatal("open isolated Fabric test schema failed")
	}
	db.SetMaxOpenConns(8)
	client := fabricent.NewClient(fabricent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	store := &PostgresOperationStore{db: db, client: client}
	t.Cleanup(func() {
		_ = client.Close()
		dropTestSchema(admin, schema)
		_ = admin.Close()
	})
	return store, db
}

func boundedPostgresURL(rawURL, schema string) string {
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("connect_timeout", "10")
		query.Set("statement_timeout", "15000")
		query.Set("application_name", "opl_fabric_catalog_test")
		if schema != "" {
			query.Set("search_path", schema)
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	result := strings.TrimSpace(rawURL) + " connect_timeout=10 statement_timeout=15000 application_name=opl_fabric_catalog_test"
	if schema != "" {
		result += " search_path=" + pq.QuoteLiteral(schema)
	}
	return result
}

func dropTestSchema(admin *sql.DB, schema string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE")
}

func randomHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal("generate Fabric test schema name failed")
	}
	return hex.EncodeToString(value)
}

func assertImmutableUpdate(t *testing.T, ctx context.Context, db *sql.DB, statement string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(ctx, statement, args...)
	var postgresError *pq.Error
	if !errors.As(err, &postgresError) || postgresError.Code != "23514" {
		if postgresError != nil {
			t.Fatalf("immutable UPDATE SQLSTATE = %s, want 23514", postgresError.Code)
		}
		t.Fatal("immutable UPDATE did not return a PostgreSQL error, want SQLSTATE 23514")
	}
}

func installCatalogInsertBarrier(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
CREATE OR REPLACE FUNCTION fabric_test_catalog_insert_barrier() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.connector_id LIKE 'concurrent-%' THEN
    PERFORM pg_advisory_xact_lock(72411002);
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER fabric_test_catalog_insert_barrier BEFORE INSERT ON fabric_connectors FOR EACH ROW EXECUTE FUNCTION fabric_test_catalog_insert_barrier();`)
	if err != nil {
		t.Fatalf("install catalog insert barrier: %v", err)
	}
}

func runConcurrentConnectorSeeds(t *testing.T, ctx context.Context, db *sql.DB, store *PostgresOperationStore, left, right Connector) [2]error {
	t.Helper()
	lockConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open advisory lock connection: %v", err)
	}
	defer lockConn.Close()
	if _, err := lockConn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", catalogInsertBarrierKey); err != nil {
		t.Fatalf("lock catalog insert barrier: %v", err)
	}
	locked := true
	defer func() {
		if locked {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = lockConn.ExecContext(cleanupCtx, "SELECT pg_advisory_unlock($1)", catalogInsertBarrierKey)
		}
	}()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, record := range []Connector{left, right} {
		record := record
		go func() {
			<-start
			results <- store.SeedCatalog(ctx, []Connector{record}, nil)
		}()
	}
	close(start)
	waitForCatalogInsertWaiters(t, ctx, db, 2)
	if _, err := lockConn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", catalogInsertBarrierKey); err != nil {
		t.Fatalf("unlock catalog insert barrier: %v", err)
	}
	locked = false
	return [2]error{<-results, <-results}
}

func waitForCatalogInsertWaiters(t *testing.T, ctx context.Context, db *sql.DB, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND granted = false AND objid = $1`, catalogInsertBarrierKey).Scan(&count)
		if err == nil && count >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("concurrent catalog inserts did not reach advisory barrier")
}

func catalogConnectorFixture(id, name string) Connector {
	connectors, _ := defaultCatalogRecords()
	record := connectors[0]
	record.ID = id
	record.VersionIdentity = id + "@" + record.Version
	record.Name = name
	record.Digest = catalogRecordDigest(record.ID, record.Version, record.VersionIdentity, record.Name, record.Status, record.ReadOnly, record.Provider, record.Resources, record.Runtime, record.CreatedAt)
	return record
}

func assertConnectorCount(t *testing.T, ctx context.Context, db *sql.DB, id, version string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM fabric_connectors WHERE connector_id = $1 AND version = $2", id, version).Scan(&count); err != nil {
		t.Fatalf("count connector rows: %v", err)
	}
	if count != want {
		t.Fatalf("connector row count = %d, want %d", count, want)
	}
}
