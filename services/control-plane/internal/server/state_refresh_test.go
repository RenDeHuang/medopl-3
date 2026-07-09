package server

import (
	"context"
	"strings"
	"testing"
)

type staticStateStore struct {
	state controlPlaneState
}

func (s *staticStateStore) Load(context.Context) (controlPlaneState, error) {
	return s.state, nil
}

func (s *staticStateStore) Save(context.Context, controlPlaneState) error {
	return nil
}

func TestApplyFactsTreatsEmptyTablesAsBackendTruth(t *testing.T) {
	app := newControlPlaneAppEmpty()
	app.computes["stale-compute"] = map[string]any{"id": "stale-compute", "status": "running"}

	app.applyFacts(controlPlaneState{
		Computes: controlPlaneRecordSet{},
		Users:    controlPlaneRecordSet{"usr-admin": {"id": "usr-admin", "email": "admin@medopl.cn", "accountId": "acct-admin", "role": "admin", "status": "active"}},
	})

	if len(app.computes) != 0 {
		t.Fatalf("empty backend compute table must clear stale current state: %#v", app.computes)
	}
}

func TestRefreshFactsFromStoreReplacesStaleCurrentState(t *testing.T) {
	app := newControlPlaneAppEmpty()
	app.computes["stale-compute"] = map[string]any{"id": "stale-compute", "status": "running"}
	app.store = &staticStateStore{state: controlPlaneState{
		Computes: controlPlaneRecordSet{"compute-fresh": {"id": "compute-fresh", "status": "running"}},
		Users:    controlPlaneRecordSet{"usr-admin": {"id": "usr-admin", "email": "admin@medopl.cn", "accountId": "acct-admin", "role": "admin", "status": "active"}},
	}}

	if err := app.refreshFactsFromStore(context.Background()); err != nil {
		t.Fatalf("refresh facts: %v", err)
	}
	if app.computes["stale-compute"] != nil || app.computes["compute-fresh"] == nil {
		t.Fatalf("refresh did not replace current state with backend facts: %#v", app.computes)
	}
}

func TestStateStoreFromEnvRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	store, err := StateStoreFromEnv()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || store != nil {
		t.Fatalf("StateStoreFromEnv must fail closed without DATABASE_URL: store=%#v err=%v", store, err)
	}
}
