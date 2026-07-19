package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opl-cloud/services/control-plane/internal/controlplane"
)

type failingSessionSaveStore struct {
	*memoryTableStore
}

func (store *failingSessionSaveStore) SaveSession(context.Context, map[string]any) error {
	return errors.New("session save failed")
}

func TestSessionCredentialVaultUsesHashedKeysAndExpiresCredentials(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	vault := newSessionCredentialVault(func() time.Time { return now })
	credential := SessionDelegatedCredential{Bearer: "delegated-user-secret", ExpiresAt: now.Add(time.Hour)}
	if err := vault.Put("raw-session-id", credential); err == nil {
		t.Fatal("raw session ID accepted as a credential key")
	}
	key := sessionLookupKey("raw-session-id")
	if err := vault.Put(key, credential); err != nil {
		t.Fatal(err)
	}
	if got, ok := vault.Get(key); !ok || got != credential {
		t.Fatalf("Get() = %#v, %v", got, ok)
	}
	now = credential.ExpiresAt
	if _, ok := vault.Get(key); ok {
		t.Fatal("expired delegated credential remained available")
	}
}

func TestVaultMissRequiresLoginAndClearsSession(t *testing.T) {
	store := newMemoryTableStore()
	remote := newIdentityTestSub2API()
	service := controlplane.NewService(fakeLedgerClient{}, &fakeFabricClient{}, remote)
	server, err := NewPersistentServer(service, store)
	if err != nil {
		t.Fatal(err)
	}
	operator := operatorSessionForTest(t, server)
	created := requestWithSession(t, server, operator, http.MethodPost, "/api/users", `{"email":"vault-miss@example.com","accountId":"acct-vault-miss","password":"CorrectHorseBatteryStaple!"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	login := loginForTest(t, server, "vault-miss@example.com", "CorrectHorseBatteryStaple!")
	cookie := login.Result().Cookies()[0]

	restarted, err := NewPersistentServer(service, store)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	restarted.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "reauthentication_required") || !strings.Contains(rec.Header().Get("Set-Cookie"), sessionCookieName+"=;") {
		t.Fatalf("vault miss status=%d cookie=%q body=%s", rec.Code, rec.Header().Get("Set-Cookie"), rec.Body.String())
	}
	sessions, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sessions[sessionLookupKey(cookie.Value)] != nil {
		t.Fatalf("vault miss left database session: %#v", sessions)
	}
}

func TestDelegatedCredentialNeverPersistsOrLeaks(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	operator := operatorSessionForTest(t, server)
	requestWithSession(t, server, operator, http.MethodPost, "/api/users", `{"email":"delegated@example.com","accountId":"acct-delegated","password":"CorrectHorseBatteryStaple!"}`)
	login := loginForTest(t, server, "delegated@example.com", "CorrectHorseBatteryStaple!")
	cookie := login.Result().Cookies()[0]
	app := server.(*controlPlaneHTTPHandler).app
	credential, ok := app.sessionCredentials.Get(sessionLookupKey(cookie.Value))
	if !ok || credential.Bearer != "test-user-delegated-token" {
		t.Fatalf("delegated credential = %#v, %v", credential, ok)
	}
	sessions, err := app.tables.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encodedSessions, err := json.Marshal(sessions)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{login.Body.String(), string(encodedSessions)} {
		if strings.Contains(value, credential.Bearer) || strings.Contains(value, "access_token") || strings.Contains(value, "refresh_token") {
			t.Fatalf("delegated credential leaked: %s", value)
		}
	}
}

func TestLogoutClearsCredential(t *testing.T) {
	server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
	login := operatorSessionForTest(t, server)
	cookie := login.Result().Cookies()[0]
	app := server.(*controlPlaneHTTPHandler).app
	key := sessionLookupKey(cookie.Value)
	if _, ok := app.sessionCredentials.Get(key); !ok {
		t.Fatal("login did not bind delegated credential")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	addAuth(req, login)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := app.sessionCredentials.Get(key); ok {
		t.Fatal("logout retained delegated credential")
	}
}

func TestSessionCredentialRollbackWhenVaultRejectsCredential(t *testing.T) {
	app := newControlPlaneApp()
	users, err := app.tables.ListUsers(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.createSession(findRecord(users, "usr-admin"), ""); err == nil {
		t.Fatal("createSession accepted an empty delegated credential")
	}
	sessions, err := app.tables.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("failed credential bind left database session: %#v", sessions)
	}
}

func TestSessionCredentialSaveFailureRollsBackVault(t *testing.T) {
	store := &failingSessionSaveStore{memoryTableStore: newMemoryTableStore()}
	app, err := newControlPlaneAppWithStore(store)
	if err != nil {
		t.Fatal(err)
	}
	users, err := app.tables.ListUsers(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.createSession(findRecord(users, "usr-admin"), "delegated-user-secret"); err == nil {
		t.Fatal("createSession succeeded when Session persistence failed")
	}
	app.sessionCredentials.mu.Lock()
	defer app.sessionCredentials.mu.Unlock()
	if len(app.sessionCredentials.credentials) != 0 {
		t.Fatal("Session persistence failure retained delegated credential")
	}
}

func TestSessionCredentialLifecycleRevokesVault(t *testing.T) {
	for _, action := range []string{"disable", "delete"} {
		t.Run(action, func(t *testing.T) {
			server := NewServer(newTestService(fakeLedgerClient{}, &fakeFabricClient{}))
			operator := operatorSessionForTest(t, server)
			created := createResourceWithSession(t, server, operator, http.MethodPost, "/api/users", `{"email":"lifecycle-`+action+`@example.com","accountId":"acct-lifecycle-`+action+`","password":"CorrectHorseBatteryStaple!"}`)
			login := loginForTest(t, server, "lifecycle-"+action+"@example.com", "CorrectHorseBatteryStaple!")
			app := server.(*controlPlaneHTTPHandler).app
			key := sessionLookupKey(login.Result().Cookies()[0].Value)
			body := `{"userId":"` + stringValue(created["id"]) + `"}`
			if action == "delete" {
				body = `{"userId":"` + stringValue(created["id"]) + `","confirm":true}`
			}
			response := requestWithSession(t, server, operator, http.MethodPost, "/api/users/"+action, body)
			if response.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", action, response.Code, response.Body.String())
			}
			if _, ok := app.sessionCredentials.Get(key); ok {
				t.Fatalf("%s retained delegated credential", action)
			}
		})
	}
}
