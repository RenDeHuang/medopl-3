package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestPasswordStrengthRejectsShortCreatePassword(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	admin := operatorSessionForTest(t, server)
	created := requestWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"weak@invite.example","accountId":"acct-weak","password":"too-short"}`)
	if created.Code != http.StatusBadRequest || !strings.Contains(created.Body.String(), "weak_password") {
		t.Fatalf("short create status=%d body=%s", created.Code, created.Body.String())
	}
	accounts, _ := server.(*controlPlaneHTTPHandler).app.tables.ListAccounts(context.Background(), "acct-weak")
	if len(accounts) != 0 {
		t.Fatalf("short create persisted account: %#v", accounts)
	}
}

func TestPasswordStrengthCountsUnicodeRunes(t *testing.T) {
	if err := validatePlaintextPassword(strings.Repeat("界", 11)); err == nil || err.Error() != "weak_password" {
		t.Fatalf("11-rune password error = %v", err)
	}
	if err := validatePlaintextPassword(strings.Repeat("界", 12)); err != nil {
		t.Fatalf("12-rune password error = %v", err)
	}
}

func TestNormalizedEmailCreateRejectsKnownCrossAccountConflictBeforeRemote(t *testing.T) {
	remote := newIdentityTestSub2API()
	server := newIdentityTestServer(t, remote, nil)
	admin := operatorSessionForTest(t, server)
	created := createResourceWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":" Owner@Invite.Example ","accountId":"acct-normalized","password":"CorrectHorseBatteryStaple!"}`)
	if created["email"] != "owner@invite.example" {
		t.Fatalf("created email = %q", created["email"])
	}
	duplicate := requestWithSession(t, server, admin, http.MethodPost, "/api/users", `{"email":"OWNER@INVITE.EXAMPLE","accountId":"acct-normalized-copy","password":"CorrectHorseBatteryStaple!"}`)
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), "user_already_exists") {
		t.Fatalf("normalized duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	if remote.resolveCalls != 1 || remote.remoteCreates != 1 {
		t.Fatalf("resolveCalls=%d remoteCreates=%d", remote.resolveCalls, remote.remoteCreates)
	}
	accounts, _ := server.(*controlPlaneHTTPHandler).app.tables.ListAccounts(context.Background(), "acct-normalized-copy")
	if len(accounts) != 0 {
		t.Fatalf("normalized duplicate persisted account: %#v", accounts)
	}
}
