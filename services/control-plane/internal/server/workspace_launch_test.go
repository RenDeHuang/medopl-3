package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkspaceLaunchOperationRoundTripsWithoutSecrets(t *testing.T) {
	input := workspaceLaunchOperation{
		ID: "launch-alpha", Status: "preparing", RequestHash: "hash", Phase: "compute",
		AccountID: "acct-alpha", OwnerUserID: "usr-alpha", WorkspaceID: "ws-alpha", Name: "Alpha", PackageID: "basic",
		StorageGB: 10, PricingVersion: "2026-07-16", TotalMonthlyPriceCNYCents: 36_800, TotalChargeUSDMicros: 52_571_429,
		ComputeID: "ca-alpha", ComputeBillingOperationID: "billing-compute-alpha",
		StorageID: "vol-alpha", StorageBillingOperationID: "billing-storage-alpha",
		AttachmentID: "attachment-alpha", AttachmentOperationID: "attach-operation-alpha", WorkspaceOperationID: "workspace-operation-alpha",
	}
	row := workspaceLaunchOperationRow(input)
	decoded, err := decodeWorkspaceLaunchOperation(row)
	if err != nil || decoded.RequestHash != input.RequestHash || decoded.ID != input.ID || decoded.Status != input.Status {
		t.Fatalf("decoded=%#v err=%v", decoded, err)
	}
	if row["action"] != "workspace.launch" || row["resourceKind"] != "workspace_launch" || row["computeAllocationId"] != input.ComputeID || row["storageId"] != input.StorageID {
		t.Fatalf("workspace launch row = %#v", row)
	}
	encoded := stringValue(row["result"])
	for _, forbidden := range []string{"password", "apiKey", "redeemCode", "rawProvider"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("encoded launch contains %q: %s", forbidden, encoded)
		}
	}
}

func TestWorkspaceLaunchResponseAllowsOnlyCustomerSafeFields(t *testing.T) {
	operation := workspaceLaunchOperation{
		ID: "launch-alpha", Status: "retryable", RequestHash: "hash", Phase: "runtime",
		AccountID: "acct-alpha", OwnerUserID: "usr-private", WorkspaceID: "ws-alpha", Name: "Alpha", PackageID: "basic",
		StorageGB: 10, PricingVersion: "2026-07-16", TotalMonthlyPriceCNYCents: 36_800, TotalChargeUSDMicros: 52_571_429,
		ComputeID: "ca-alpha", ComputeBillingOperationID: "billing-compute-private",
		StorageID: "vol-alpha", StorageBillingOperationID: "billing-storage-private",
		AttachmentID: "attachment-alpha", AttachmentOperationID: "attachment-operation-private", WorkspaceOperationID: "workspace-operation-private",
		ErrorCode: "upstream_unavailable",
	}
	row := workspaceLaunchOperationRow(operation)
	var persisted map[string]any
	if err := json.Unmarshal([]byte(stringValue(row["result"])), &persisted); err != nil {
		t.Fatal(err)
	}
	persisted["dependencyError"] = "private upstream detail"
	persisted["password"] = "private-password"
	encoded, err := json.Marshal(persisted)
	if err != nil {
		t.Fatal(err)
	}
	row["result"] = string(encoded)
	row["internalDependencyError"] = "private row detail"

	response, err := workspaceLaunchResponse(row)
	if err != nil {
		t.Fatal(err)
	}
	if response["operationId"] != operation.ID || response["status"] != operation.Status || response["phase"] != operation.Phase || response["errorCode"] != operation.ErrorCode {
		t.Fatalf("workspace launch response = %#v", response)
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"usr-private", "billing-compute-private", "billing-storage-private", "attachment-operation-private", "workspace-operation-private", "private upstream detail", "private-password", "private row detail"} {
		if strings.Contains(string(responseJSON), forbidden) {
			t.Fatalf("workspace launch response leaked %q: %s", forbidden, responseJSON)
		}
	}
}
