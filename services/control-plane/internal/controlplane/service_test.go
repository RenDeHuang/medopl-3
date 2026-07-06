package controlplane

import (
	"context"
	"reflect"
	"testing"

	"opl-cloud/services/control-plane/internal/clients"
)

func TestCreateWorkspaceOrchestratesLedgerAndFabric(t *testing.T) {
	calls := []string{}
	ledger := &fakeLedgerClient{calls: &calls}
	fabric := &fakeFabricClient{calls: &calls}
	service := NewService(ledger, fabric)

	workspace, err := service.CreateWorkspace(context.Background(), CreateWorkspaceInput{
		AccountID: "acct-alpha",
		OwnerID:   "usr-owner",
		Name:      "Alpha Lab",
		PackageID: "basic",
	}, "workspace-once")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if workspace.ID == "" {
		t.Fatalf("expected workspace id")
	}
	if workspace.URL == "" {
		t.Fatalf("expected runtime url")
	}
	wantCalls := []string{"ledger.hold", "fabric.compute", "fabric.storage", "fabric.runtime", "ledger.evidence"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

type fakeLedgerClient struct {
	calls *[]string
}

func (f *fakeLedgerClient) CreateHold(ctx context.Context, input clients.HoldInput, idempotencyKey string) (clients.HoldResult, error) {
	*f.calls = append(*f.calls, "ledger.hold")
	return clients.HoldResult{ID: "hold-alpha", AccountID: input.AccountID, AmountCents: 1000}, nil
}

func (f *fakeLedgerClient) RecordEvidence(ctx context.Context, input clients.EvidenceInput, idempotencyKey string) (clients.EvidenceReceipt, error) {
	*f.calls = append(*f.calls, "ledger.evidence")
	return clients.EvidenceReceipt{ID: "ev-alpha", WorkspaceID: input.WorkspaceID}, nil
}

type fakeFabricClient struct {
	calls *[]string
}

func (f *fakeFabricClient) CreateComputeAllocation(ctx context.Context, input clients.ComputeAllocationInput, idempotencyKey string) (clients.ComputeAllocation, error) {
	*f.calls = append(*f.calls, "fabric.compute")
	return clients.ComputeAllocation{ID: "compute-alpha", ProviderRequestID: "compute-request-alpha"}, nil
}

func (f *fakeFabricClient) CreateStorageVolume(ctx context.Context, input clients.StorageVolumeInput, idempotencyKey string) (clients.StorageVolume, error) {
	*f.calls = append(*f.calls, "fabric.storage")
	return clients.StorageVolume{ID: "volume-alpha", ProviderRequestID: "volume-request-alpha"}, nil
}

func (f *fakeFabricClient) CreateWorkspaceRuntime(ctx context.Context, input clients.WorkspaceRuntimeInput, idempotencyKey string) (clients.WorkspaceRuntime, error) {
	*f.calls = append(*f.calls, "fabric.runtime")
	return clients.WorkspaceRuntime{ID: "runtime-alpha", WorkspaceID: input.WorkspaceID, URL: "https://workspace.medopl.cn/w/" + input.WorkspaceID + "/"}, nil
}
