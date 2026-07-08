package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"opl-cloud/services/fabric/internal/fabric"
)

func TestCatalogHTTP(t *testing.T) {
	server := NewServer(fabric.NewService(testProvider{}))
	req := httptest.NewRequest(http.MethodGet, "/fabric/catalog", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var catalog fabric.Catalog
	if err := json.NewDecoder(rec.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalog.WorkspacePackages) == 0 {
		t.Fatalf("expected workspace packages")
	}
}

func TestCreateComputeAllocationHTTPRequiresIdempotencyKey(t *testing.T) {
	server := NewServer(fabric.NewService(testProvider{}))
	body := bytes.NewBufferString(`{"accountId":"acct-alpha","workspaceId":"ws-alpha","packageId":"basic","dryRun":true}`)
	req := httptest.NewRequest(http.MethodPost, "/fabric/compute-allocations", body)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestOperationsHTTPReturnsFabricAuditFacts(t *testing.T) {
	service := fabric.NewService(testProvider{})
	server := NewServer(service)

	create := httptest.NewRequest(http.MethodPost, "/fabric/storage-volumes", bytes.NewBufferString(`{"accountId":"acct-alpha","workspaceId":"ws-alpha","sizeGb":10}`))
	create.Header.Set("Idempotency-Key", "http-ops-storage")
	createRec := httptest.NewRecorder()
	server.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d: %s", createRec.Code, http.StatusAccepted, createRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/fabric/operations", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("operations status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var operations []fabric.FabricOperation
	if err := json.NewDecoder(rec.Body).Decode(&operations); err != nil {
		t.Fatalf("decode operations: %v", err)
	}
	for _, operation := range operations {
		if operation.Action == "create_storage_volume" && operation.ResourceKind == "storage_volume" && operation.Status == "succeeded" {
			if operation.OperationID == "" || operation.ProviderRequestID != "storage-test" || operation.RequestHash == "" {
				t.Fatalf("operation missing audit identity: %#v", operation)
			}
			return
		}
	}
	t.Fatalf("missing storage operation in %#v", operations)
}

type testProvider struct{}

func (testProvider) CreateComputeAllocation(_ context.Context, input fabric.ComputeAllocationInput) (fabric.ComputeAllocation, error) {
	return fabric.ComputeAllocation{ID: "ca-test", AccountID: input.AccountID, WorkspaceID: input.WorkspaceID, PackageID: input.PackageID, Status: "allocated", ProviderRequestID: "compute-test"}, nil
}

func (testProvider) DestroyComputeAllocation(_ context.Context, allocation fabric.ComputeAllocation) (fabric.ComputeAllocation, error) {
	allocation.Status = "destroyed"
	return allocation, nil
}

func (testProvider) CreateStorageVolume(_ context.Context, input fabric.StorageVolumeInput) (fabric.StorageVolume, error) {
	return fabric.StorageVolume{ID: "vol-test", AccountID: input.AccountID, WorkspaceID: input.WorkspaceID, Status: "ready", ProviderRequestID: "storage-test"}, nil
}

func (testProvider) DestroyStorageVolume(_ context.Context, volume fabric.StorageVolume) (fabric.StorageVolume, error) {
	volume.Status = "destroyed"
	return volume, nil
}

func (testProvider) CreateStorageAttachment(_ context.Context, input fabric.StorageAttachmentInput, _ fabric.ComputeAllocation, _ fabric.StorageVolume) (fabric.StorageAttachment, error) {
	return fabric.StorageAttachment{ID: "att-test", WorkspaceID: input.WorkspaceID, ComputeID: input.ComputeID, VolumeID: input.VolumeID, Status: "attached", ProviderRequestID: "attachment-test"}, nil
}

func (testProvider) DetachStorageAttachment(_ context.Context, attachment fabric.StorageAttachment) (fabric.StorageAttachment, error) {
	attachment.Status = "detached"
	return attachment, nil
}

func (testProvider) CreateWorkspaceRuntime(_ context.Context, input fabric.WorkspaceRuntimeInput, _ fabric.ComputeAllocation, _ fabric.StorageVolume) (fabric.WorkspaceRuntime, error) {
	return fabric.WorkspaceRuntime{ID: "rt-test", WorkspaceID: input.WorkspaceID, Status: "running", ProviderRequestID: "runtime-test"}, nil
}

func (testProvider) WorkspaceRuntimeStatus(_ context.Context, workspaceID string) (fabric.WorkspaceRuntime, error) {
	return fabric.WorkspaceRuntime{WorkspaceID: workspaceID, Status: "not_found"}, nil
}

func (testProvider) Readiness(_ context.Context) (map[string]any, error) {
	return map[string]any{"provider": "test", "ready": true}, nil
}
