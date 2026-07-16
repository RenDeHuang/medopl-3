package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"opl-cloud/services/control-plane/internal/clients"
)

func TestRuntimeStatusNeverReturnsCredential(t *testing.T) {
	store := newMemoryTableStore()
	fabric := &fakeFabricClient{runtimeStatus: clients.WorkspaceRuntime{
		ID: "runtime-alpha", WorkspaceID: "ws-alpha", Status: "running", Ready: true,
		Access: clients.WorkspaceRuntimeAccess{
			Username: "opl", Password: "runtime-password-alpha", CredentialStatus: "configured",
			CredentialVersion: "v1", SecretRef: "runtime-secret-alpha",
		},
	}}
	server, err := NewPersistentServer(newTestService(fakeLedgerClient{}, fabric), store)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	owner := tenantOwnerSessionForTest(t, server)
	member := tenantSessionForTest(t, server, "member")
	mustStore(t, store.SaveWorkspace(context.Background(), map[string]any{
		"id": "ws-alpha", "accountId": "acct-alpha", "ownerAccountId": "acct-alpha",
		"ownerUserId": sessionUserIDForTest(t, server, owner), "state": "running", "status": "running",
	}))

	response := requestWithSession(t, server, member, http.MethodPost, "/api/workspaces/runtime-status", `{"workspaceId":"ws-alpha"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("runtime status = %d: %s", response.Code, response.Body.String())
	}
	for _, secret := range []string{"runtime-password-alpha", `"password"`, `"secretRef"`} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatalf("runtime status leaked %q: %s", secret, response.Body.String())
		}
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
	stored, err := store.ListWorkspaces(context.Background(), "acct-alpha")
	if err != nil || len(stored) != 1 || nested(stored[0], "access", "password") != nil {
		t.Fatalf("stored Workspace leaked password: rows=%#v err=%v", stored, err)
	}
}
