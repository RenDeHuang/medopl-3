package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestInvitedAccountCreatesAccountUserOrganizationAndOwnerMembership(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	admin := operatorSessionForTest(t, server)
	user := createResourceWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"owner@invite.example","accountId":"acct-invite","password":"CorrectHorseBatteryStaple!","sub2apiUserId":73}`)

	app := server.(*controlPlaneHTTPHandler).app
	accounts, _ := app.tables.ListAccounts(context.Background(), "acct-invite")
	organizations, _ := app.tables.ListOrganizations(context.Background())
	memberships, _ := app.tables.ListMemberships(context.Background())
	organization := findRecord(organizations, "org-invite")

	if account := findRecord(accounts, "acct-invite"); account == nil || int64(numberField(account, "sub2apiUserId", 0)) != 73 {
		t.Fatalf("invited account = %#v", account)
	}
	if user["accountId"] != "acct-invite" || user["role"] != "owner" {
		t.Fatalf("invited user = %#v", user)
	}
	if organization == nil || organization["billingAccountId"] != "acct-invite" {
		t.Fatalf("invited organization = %#v", organization)
	}
	membership := findRecord(memberships, "mem-"+stableID("org-invite", stringValue(user["id"]))[:12])
	if membership == nil || membership["accountId"] != "acct-invite" || membership["userId"] != user["id"] || membership["role"] != "owner" || membership["status"] != "active" {
		t.Fatalf("invited membership = %#v", membership)
	}
}

func TestMemoryInvitedAccountRollsBackEveryValidationStage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seed    func(*memoryTableStore)
		mutate  func(map[string]any, map[string]any, map[string]any, map[string]any)
		wantErr error
	}{
		{
			name: "account mapping",
			seed: func(store *memoryTableStore) {
				mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-existing", "status": "active", "sub2apiUserId": int64(73)}))
			},
			wantErr: errSub2APIAccountMappingConflict,
		},
		{
			name: "normalized user email",
			seed: func(store *memoryTableStore) {
				mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-existing", "status": "active", "sub2apiUserId": int64(74)}))
				mustStore(t, store.SaveUser(context.Background(), map[string]any{"id": "usr-existing", "email": "owner@invite.example", "accountId": "acct-existing", "role": "owner", "status": "active"}))
			},
			mutate: func(_ map[string]any, user, _, _ map[string]any) {
				user["email"] = " OWNER@INVITE.EXAMPLE "
			},
			wantErr: errUserExists,
		},
		{
			name: "organization billing account",
			seed: func(store *memoryTableStore) {
				mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-existing", "status": "active", "sub2apiUserId": int64(74)}))
				mustStore(t, store.SaveOrganization(context.Background(), map[string]any{"id": "org-invite", "billingAccountId": "acct-existing", "status": "active"}))
			},
			wantErr: errMembershipAccountMismatch,
		},
		{
			name: "membership relationship",
			mutate: func(_, _, _, membership map[string]any) {
				membership["userId"] = "usr-other"
			},
			wantErr: errMembershipUserNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemoryTableStore()
			if tc.seed != nil {
				tc.seed(store)
			}
			account, user, organization, membership := invitedAccountRows()
			if tc.mutate != nil {
				tc.mutate(account, user, organization, membership)
			}
			before := []controlPlaneRecordSet{
				cloneStateTable(store.accounts), cloneStateTable(store.users),
				cloneStateTable(store.organizations), cloneStateTable(store.memberships),
			}

			err := store.CreateInvitedAccount(context.Background(), account, user, organization, membership)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateInvitedAccount error = %v, want %v", err, tc.wantErr)
			}
			after := []controlPlaneRecordSet{store.accounts, store.users, store.organizations, store.memberships}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("partial invited account rows remain: before=%#v after=%#v", before, after)
			}
		})
	}
}

func invitedAccountRows() (map[string]any, map[string]any, map[string]any, map[string]any) {
	account := map[string]any{"id": "acct-invite", "status": "active", "sub2apiUserId": int64(73)}
	user := map[string]any{"id": "usr-invite", "email": "owner@invite.example", "accountId": "acct-invite", "role": "owner", "status": "active", "passwordHash": "hash"}
	organization := map[string]any{"id": "org-invite", "name": "Organization acct-invite", "billingAccountId": "acct-invite", "status": "active"}
	membership := map[string]any{"id": "mem-" + stableID("org-invite", "usr-invite")[:12], "accountId": "acct-invite", "organizationId": "org-invite", "userId": "usr-invite", "role": "owner", "status": "active"}
	return account, user, organization, membership
}

func TestEntInvitedAccountAcceptsMatchingAccountAndOrganization(t *testing.T) {
	ctx := context.Background()
	store := NewTestEntStateStore(t, t.TempDir()+"/invited-existing.sqlite").(*postgresEntStateStore)
	account, user, organization, membership := invitedAccountRows()
	mustStore(t, store.SaveAccount(ctx, account))
	mustStore(t, store.SaveOrganization(ctx, organization))

	if err := store.CreateInvitedAccount(ctx, account, user, organization, membership); err != nil {
		t.Fatal(err)
	}
	accounts, _ := store.ListAccounts(ctx, "acct-invite")
	organizations, _ := store.ListOrganizations(ctx)
	users, _ := store.ListUsers(ctx, true)
	memberships, _ := store.ListMemberships(ctx)
	if len(accounts) != 1 || len(organizations) != 1 || findRecord(users, "usr-invite") == nil || findRecord(memberships, stringValue(membership["id"])) == nil {
		t.Fatalf("matching invite facts: accounts=%#v organizations=%#v users=%#v memberships=%#v", accounts, organizations, users, memberships)
	}
}

func TestEntInvitedAccountRollsBackOnMembershipInsertError(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/invited-rollback.sqlite"
	store := NewTestEntStateStore(t, path).(*postgresEntStateStore)
	db, err := sql.Open("sqlite3", path+"?_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TRIGGER fail_invited_membership BEFORE INSERT ON control_plane_memberships BEGIN SELECT RAISE(ABORT, 'membership insert failed'); END`); err != nil {
		t.Fatal(err)
	}
	account, user, organization, membership := invitedAccountRows()

	if err := store.CreateInvitedAccount(ctx, account, user, organization, membership); err == nil {
		t.Fatal("CreateInvitedAccount error = nil")
	}
	accounts, _ := store.ListAccounts(ctx, "acct-invite")
	organizations, _ := store.ListOrganizations(ctx)
	users, _ := store.ListUsers(ctx, true)
	memberships, _ := store.ListMemberships(ctx)
	if len(accounts) != 0 || findRecord(organizations, "org-invite") != nil || findRecord(users, "usr-invite") != nil || findRecord(memberships, stringValue(membership["id"])) != nil {
		t.Fatalf("partial Ent invite survived rollback: accounts=%#v organizations=%#v users=%#v memberships=%#v", accounts, organizations, users, memberships)
	}
}

func TestPostgresInvitedAccountSerializesExistingAccountOrganization(t *testing.T) {
	databaseURL := os.Getenv("CONTROL_PLANE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CONTROL_PLANE_TEST_DATABASE_URL is not set")
	}
	admin, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Ping(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("control_plane_invite_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + pq.QuoteIdentifier(schema) + ` CASCADE`) })

	stateStore, err := NewPostgresEntStateStore(postgresInvitedAccountTestURL(databaseURL, schema))
	if err != nil {
		t.Fatal(err)
	}
	store := stateStore.(*postgresEntStateStore)
	t.Cleanup(func() { _ = store.client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	account, firstUser, organization, firstMembership := invitedAccountRows()
	mustStore(t, store.SaveAccount(ctx, account))
	if _, err := admin.Exec(`
		CREATE FUNCTION ` + pq.QuoteIdentifier(schema) + `.delay_invited_account_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END
		$$;
		CREATE TRIGGER delay_invited_account_update BEFORE UPDATE ON ` + pq.QuoteIdentifier(schema) + `.control_plane_accounts
		FOR EACH ROW EXECUTE FUNCTION ` + pq.QuoteIdentifier(schema) + `.delay_invited_account_update();
	`); err != nil {
		t.Fatal(err)
	}
	secondUser := cloneMap(firstUser)
	secondUser["id"], secondUser["email"] = "usr-invite-two", "owner-two@invite.example"
	secondMembership := cloneMap(firstMembership)
	secondMembership["id"], secondMembership["userId"] = "mem-invite-two", secondUser["id"]

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, invite := range [][2]map[string]any{{firstUser, firstMembership}, {secondUser, secondMembership}} {
		go func(user, membership map[string]any) {
			<-start
			results <- store.CreateInvitedAccount(ctx, account, user, organization, membership)
		}(invite[0], invite[1])
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent invited account: %v", err)
		}
	}
	organizations, _ := store.ListOrganizations(ctx)
	users, _ := store.ListUsers(ctx, true)
	memberships, _ := store.ListMemberships(ctx)
	if len(organizations) != 1 || len(users) != 2 || len(memberships) != 2 {
		t.Fatalf("concurrent invite facts: organizations=%#v users=%#v memberships=%#v", organizations, users, memberships)
	}
}

func postgresInvitedAccountTestURL(databaseURL, schema string) string {
	if parsed, err := url.Parse(databaseURL); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return databaseURL + " search_path=" + pq.QuoteLiteral(schema)
}

func TestEntUserLifecycleRollsBackAllFacts(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/user-lifecycle-rollback.sqlite"
	store := NewTestEntStateStore(t, path).(*postgresEntStateStore)
	user := map[string]any{"id": "usr-lifecycle", "email": "lifecycle@example.com", "accountId": "acct-lifecycle", "role": "owner", "status": "active"}
	mustStore(t, store.SaveAccount(ctx, map[string]any{"id": "acct-lifecycle", "status": "active", "sub2apiUserId": int64(113)}))
	mustStore(t, store.SaveUser(ctx, user))
	mustStore(t, store.SaveSession(ctx, map[string]any{"id": "session-lifecycle", "userId": "usr-lifecycle", "csrf": "csrf", "expiresAt": "2099-01-01T00:00:00Z"}))
	mustStore(t, store.SaveCompute(ctx, map[string]any{"id": "compute-lifecycle", "accountId": "acct-lifecycle", "autoRenew": true}))
	mustStore(t, store.SaveStorage(ctx, map[string]any{"id": "storage-lifecycle", "accountId": "acct-lifecycle", "autoRenew": true}))
	db, err := sql.Open("sqlite3", path+"?_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TRIGGER fail_lifecycle_storage BEFORE UPDATE ON control_plane_storage_volumes BEGIN SELECT RAISE(ABORT, 'storage update failed'); END`); err != nil {
		t.Fatal(err)
	}
	user["status"] = "disabled"

	if err := store.ApplyUserLifecycle(ctx, user); err == nil {
		t.Fatal("ApplyUserLifecycle error = nil")
	}
	users, _ := store.ListUsers(ctx, true)
	sessions, _ := store.ListSessions(ctx)
	computes, _ := store.ListComputes(ctx, "acct-lifecycle")
	storages, _ := store.ListStorages(ctx, "acct-lifecycle")
	if findRecord(users, "usr-lifecycle")["status"] != "active" || sessions["session-lifecycle"] == nil || findRecord(computes, "compute-lifecycle")["autoRenew"] != true || findRecord(storages, "storage-lifecycle")["autoRenew"] != true {
		t.Fatalf("partial lifecycle survived rollback: users=%#v sessions=%#v computes=%#v storages=%#v", users, sessions, computes, storages)
	}
}
