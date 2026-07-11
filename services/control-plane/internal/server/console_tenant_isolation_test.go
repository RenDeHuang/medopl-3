package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/controlplane"
)

func TestBootstrapUsersUseOnlyFixedRoles(t *testing.T) {
	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"email":"owner@example.com","password":"correct horse battery staple"}]`)
	users, err := bootstrapUsersFromEnv()
	if err != nil {
		t.Fatalf("bootstrap users: %v", err)
	}
	if got := stringValue(users[0]["role"]); got != "owner" {
		t.Fatalf("default role = %q, want owner", got)
	}

	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"email":"pi@example.com","password":"correct horse battery staple","role":"pi"}]`)
	if _, err := bootstrapUsersFromEnv(); err == nil || !strings.Contains(err.Error(), "invalid_role") {
		t.Fatalf("invalid explicit role error = %v, want invalid_role", err)
	}
}

func TestOrganizationRejectsMissingBillingAccount(t *testing.T) {
	app := newControlPlaneApp()
	if _, err := app.createOrganization(map[string]any{"name": "Orphan", "billingAccountId": "acct-missing"}); err == nil {
		t.Fatal("organization with missing billing account was accepted")
	}
}

func TestMembershipRejectsMissingReferences(t *testing.T) {
	app := newControlPlaneApp()
	if _, err := app.createMembership(map[string]any{"organizationId": "org-missing", "userId": "usr-missing", "accountId": "acct-missing", "role": "member"}); err == nil {
		t.Fatal("membership with missing references was accepted")
	}
}

func TestBackendRejectsRolesOutsideOwnerAdminMember(t *testing.T) {
	app := newControlPlaneApp()
	mustStore(t, app.tables.SaveAccount(context.Background(), map[string]any{"id": "acct-alpha", "status": "active"}))
	mustStore(t, app.tables.SaveOrganization(context.Background(), map[string]any{"id": "org-alpha", "billingAccountId": "acct-alpha", "status": "active"}))
	mustStore(t, app.tables.SaveUser(context.Background(), map[string]any{"id": "usr-member", "email": "member@example.com", "accountId": "acct-alpha", "role": "member", "status": "active"}))
	for _, role := range []string{"pi", "viewer", "OWNER"} {
		if _, err := app.createUser(map[string]any{"email": role + "@example.com", "accountId": "acct-alpha", "role": role, "password": "correct horse battery staple"}); err == nil || err.Error() != "invalid_role" {
			t.Fatalf("user role %q error = %v, want invalid_role", role, err)
		}
		if _, err := app.createMembership(map[string]any{"organizationId": "org-alpha", "userId": "usr-member", "accountId": "acct-alpha", "role": role}); err == nil || err.Error() != "invalid_role" {
			t.Fatalf("membership role %q error = %v, want invalid_role", role, err)
		}
	}
}

func TestAdminCannotUseCustomerEndpointsAcrossAccountsOrOrganizations(t *testing.T) {
	app := newControlPlaneApp()
	req := requestForStoredUser(t, app, map[string]any{"id": "usr-admin", "email": "admin@example.com", "accountId": "acct-admin", "role": "admin", "status": "active"})

	rec := httptest.NewRecorder()
	if _, ok := app.scopedAccountID(rec, req, map[string]any{"accountId": "acct-other"}); ok || rec.Code != http.StatusForbidden {
		t.Fatalf("cross-account admin scope status=%d ok=%v, want 403 false", rec.Code, ok)
	}
	if app.canAccessResource(req, map[string]any{"id": "ws-other", "accountId": "acct-other"}) {
		t.Fatal("admin accessed another account resource through a customer endpoint")
	}

	rec = httptest.NewRecorder()
	if app.authorizeOrganization(rec, req, "org-other") || rec.Code != http.StatusForbidden {
		t.Fatalf("cross-organization admin status=%d, want 403", rec.Code)
	}
}

type crossTenantComputeFabric struct{ fakeFabricClient }

func (crossTenantComputeFabric) GetComputeAllocation(_ context.Context, id string) (clients.ComputeAllocation, error) {
	return clients.ComputeAllocation{ID: id, AccountID: "acct-beta", Status: "running"}, nil
}

func TestFreshComputeReadRejectsCrossTenantProjection(t *testing.T) {
	server := NewServer(controlplane.NewService(fakeLedgerClient{}, &crossTenantComputeFabric{}))
	req := httptest.NewRequest(http.MethodGet, "/api/compute-allocations/compute-beta", nil)
	addSessionCookies(req, operatorSessionForTest(t, server))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fresh cross-tenant compute status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestMembershipRevocationAndUserDisableTakeEffectImmediately(t *testing.T) {
	app := newControlPlaneApp()
	user := map[string]any{"id": "usr-member", "email": "member@example.com", "accountId": "acct-alpha", "role": "member", "status": "active"}
	mustStore(t, app.tables.SaveAccount(context.Background(), map[string]any{"id": "acct-alpha", "status": "active"}))
	mustStore(t, app.tables.SaveOrganization(context.Background(), map[string]any{"id": "org-alpha", "billingAccountId": "acct-alpha", "status": "active"}))
	req := requestForStoredUser(t, app, user)
	membership := map[string]any{"id": "mem-alpha", "organizationId": "org-alpha", "userId": "usr-member", "accountId": "acct-alpha", "role": "member", "status": "active"}
	mustStore(t, app.tables.SaveMembership(context.Background(), membership))

	if !app.authorizeOrganization(httptest.NewRecorder(), req, "org-alpha") {
		t.Fatal("active membership was denied")
	}
	membership["status"] = "revoked"
	mustStore(t, app.tables.SaveMembership(context.Background(), membership))
	rec := httptest.NewRecorder()
	if app.authorizeOrganization(rec, req, "org-alpha") || rec.Code != http.StatusForbidden {
		t.Fatalf("revoked membership status=%d, want 403", rec.Code)
	}

	user["status"] = "disabled"
	mustStore(t, app.tables.SaveUser(context.Background(), user))
	if _, ok := app.session(req); ok {
		t.Fatal("disabled user retained an active session")
	}
}

func TestPostgresLegacyMembershipMigrationIsLosslessAndFailClosed(t *testing.T) {
	admin, err := sql.Open("postgres", "host=/var/run/postgresql dbname=postgres sslmode=disable")
	if err != nil {
		t.Skipf("open local postgres: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	schema := fmt.Sprintf("control_plane_membership_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })

	db, err := sql.Open("postgres", "host=/var/run/postgresql dbname=postgres sslmode=disable search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE control_plane_accounts (id text PRIMARY KEY);
		CREATE TABLE control_plane_organizations (id text PRIMARY KEY, billing_account_id text NOT NULL);
		CREATE TABLE control_plane_users (id text PRIMARY KEY, account_id text NOT NULL);
		CREATE TABLE control_plane_memberships (id text PRIMARY KEY, account_id text NOT NULL, organization_id text NOT NULL, user_id text NOT NULL, role text NOT NULL, status text NOT NULL);
		INSERT INTO control_plane_accounts VALUES ('acct-alpha');
		INSERT INTO control_plane_organizations VALUES ('org-alpha', 'acct-alpha');
		INSERT INTO control_plane_users VALUES ('usr-owner', 'acct-alpha');
		INSERT INTO control_plane_memberships VALUES
			('mem-owner', 'acct-alpha', 'org-alpha', 'usr-owner', ' Owner ', 'active'),
			('mem-admin', 'acct-alpha', 'org-alpha', 'usr-missing', 'ADMIN', 'active');
	`); err != nil {
		t.Fatal(err)
	}
	driver := entsql.OpenDB(dialect.Postgres, db)
	if err := validateAndNormalizeLegacyMemberships(context.Background(), driver); err == nil {
		t.Fatal("migration accepted membership with missing user truth")
	}
	assertMembershipRoles(t, db, []string{"ADMIN", " Owner "})

	if _, err := db.Exec(`INSERT INTO control_plane_users VALUES ('usr-missing', 'acct-alpha')`); err != nil {
		t.Fatal(err)
	}
	if err := validateAndNormalizeLegacyMemberships(context.Background(), driver); err != nil {
		t.Fatalf("normalize valid legacy memberships: %v", err)
	}
	assertMembershipRoles(t, db, []string{"admin", "owner"})
}

func TestPostgresStoreStartsFromFreshDatabase(t *testing.T) {
	admin, err := sql.Open("postgres", "host=/var/run/postgresql dbname=postgres sslmode=disable")
	if err != nil {
		t.Skipf("open local postgres: %v", err)
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	database := fmt.Sprintf("control_plane_fresh_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE ` + database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, database)
		_, _ = admin.Exec(`DROP DATABASE ` + database)
	})

	store, err := NewPostgresEntStateStore("host=/var/run/postgresql dbname=" + database + " sslmode=disable")
	if err != nil {
		t.Fatalf("start store on fresh database: %v", err)
	}
	defer store.(*postgresEntStateStore).client.Close()
	accounts, err := store.ListAccounts(context.Background())
	if err != nil || len(accounts) != 0 {
		t.Fatalf("fresh account table = %v, err=%v", accounts, err)
	}
}

func assertMembershipRoles(t *testing.T, db *sql.DB, want []string) {
	t.Helper()
	rows, err := db.Query(`SELECT role FROM control_plane_memberships ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			t.Fatal(err)
		}
		got = append(got, role)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("membership roles = %v, want %v", got, want)
	}
}

func requestForStoredUser(t *testing.T, app *controlPlaneServer, user map[string]any) *http.Request {
	t.Helper()
	mustStore(t, app.tables.SaveUser(context.Background(), user))
	_, sessionID, err := app.createSession(user)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewBufferString(`{}`))
	req.AddCookie(sessionCookie(sessionID, 3600))
	return req
}
