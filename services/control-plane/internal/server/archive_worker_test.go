package server

import (
	"context"
	"testing"
)

func TestArchiveRetentionWorkerArchivesTerminalCurrentStateOnly(t *testing.T) {
	app := newControlPlaneAppEmpty()
	app.computes["compute-dead"] = map[string]any{"id": "compute-dead", "status": "destroyed"}
	app.storages["storage-dead"] = map[string]any{"id": "storage-dead", "status": "destroyed"}
	app.attachments["attach-dead"] = map[string]any{"id": "attach-dead", "status": "detached"}
	app.workspaces["ws-dead"] = map[string]any{"id": "ws-dead", "state": "unrecoverable"}
	app.ledger = []map[string]any{{"id": "ledger-kept"}}

	if err := app.runArchiveRetentionOnce(context.Background()); err != nil {
		t.Fatalf("run archive retention: %v", err)
	}
	if len(app.computes) != 0 || len(app.storages) != 0 || len(app.attachments) != 0 || len(app.workspaces) != 0 {
		t.Fatalf("terminal current state remains: computes=%#v storages=%#v attachments=%#v workspaces=%#v", app.computes, app.storages, app.attachments, app.workspaces)
	}
	if len(app.ledger) != 1 {
		t.Fatalf("ledger accounting facts must be retained: %#v", app.ledger)
	}
}
