package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestPasswordStrengthRejectsShortCreateResetAndBootstrapPlaintext(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	admin := operatorSessionForTest(t, server)
	created := requestWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"weak@invite.example","accountId":"acct-weak","password":"too-short","sub2apiUserId":75}`)
	if created.Code != http.StatusBadRequest || !strings.Contains(created.Body.String(), "weak_password") {
		t.Fatalf("short create status=%d body=%s", created.Code, created.Body.String())
	}
	accounts, _ := server.(*controlPlaneHTTPHandler).app.tables.ListAccounts(context.Background(), "acct-weak")
	if len(accounts) != 0 {
		t.Fatalf("short create persisted account: %#v", accounts)
	}

	user := createResourceWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"reset@invite.example","accountId":"acct-reset","password":"CorrectHorseBatteryStaple!","sub2apiUserId":76}`)
	reset := requestWithSession(t, server, admin, http.MethodPost, "/api/users/"+stringValue(user["id"])+"/reset-password", `{"password":"too-short"}`)
	if reset.Code != http.StatusBadRequest || !strings.Contains(reset.Body.String(), "weak_password") {
		t.Fatalf("short reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	if login := loginForTest(t, server, "reset@invite.example", "CorrectHorseBatteryStaple!"); login.Code != http.StatusOK {
		t.Fatalf("short reset changed password: status=%d body=%s", login.Code, login.Body.String())
	}

	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"id":"usr-bootstrap-weak","email":"bootstrap@example.com","password":"too-short","role":"owner","accountId":"acct-bootstrap","sub2apiUserId":77}]`)
	if _, err := bootstrapUsersFromEnv(); err == nil || err.Error() != "weak_password" {
		t.Fatalf("short bootstrap password error = %v", err)
	}
}

func TestPasswordStrengthDoesNotGuessBootstrapHashLength(t *testing.T) {
	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"id":"usr-bootstrap-hash","email":"bootstrap@example.com","passwordHash":"opaque","role":"owner","accountId":"acct-bootstrap","sub2apiUserId":77}]`)
	users, err := bootstrapUsersFromEnv()
	if err != nil || len(users) != 1 || users[0]["passwordHash"] != "opaque" {
		t.Fatalf("hashed bootstrap users=%#v err=%v", users, err)
	}
}

func TestNormalizedEmailCreateAndBootstrapReuseOneIdentity(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	admin := operatorSessionForTest(t, server)
	created := createResourceWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":" Owner@Invite.Example ","accountId":"acct-normalized","password":"CorrectHorseBatteryStaple!","sub2apiUserId":78}`)
	if created["email"] != "owner@invite.example" {
		t.Fatalf("created email = %q", created["email"])
	}
	duplicate := requestWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"OWNER@INVITE.EXAMPLE","accountId":"acct-normalized-copy","password":"CorrectHorseBatteryStaple!","sub2apiUserId":79}`)
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), "user_already_exists") {
		t.Fatalf("normalized duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	accounts, _ := server.(*controlPlaneHTTPHandler).app.tables.ListAccounts(context.Background(), "acct-normalized-copy")
	if len(accounts) != 0 {
		t.Fatalf("normalized duplicate persisted account: %#v", accounts)
	}

	store := newMemoryTableStore()
	hash, err := hashPassword("CorrectHorseBatteryStaple!")
	if err != nil {
		t.Fatal(err)
	}
	mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-bootstrap-normalized", "status": "active", "sub2apiUserId": int64(80)}))
	mustStore(t, store.SaveUser(context.Background(), map[string]any{"id": "usr-bootstrap-original", "email": "bootstrap@example.com", "accountId": "acct-bootstrap-normalized", "role": "owner", "status": "active", "passwordHash": hash}))
	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"id":"usr-bootstrap-copy","email":" Bootstrap@Example.Com ","password":"CorrectHorseBatteryStaple!","role":"owner","accountId":"acct-bootstrap-normalized","sub2apiUserId":80}]`)
	if _, err := newControlPlaneAppWithStore(store); err != nil {
		t.Fatal(err)
	}
	users, _ := store.ListUsers(context.Background(), true)
	matches := 0
	for _, user := range users {
		if normalizeEmail(stringValue(user["email"])) == "bootstrap@example.com" {
			matches++
			if user["email"] != "bootstrap@example.com" || user["id"] != "usr-bootstrap-original" {
				t.Fatalf("bootstrap identity = %#v", user)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("bootstrap normalized matches = %d, users=%#v", matches, users)
	}
}
