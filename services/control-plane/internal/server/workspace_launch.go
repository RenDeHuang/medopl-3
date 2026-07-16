package server

import (
	"encoding/json"
	"errors"
)

type workspaceLaunchOperation struct {
	ID                        string `json:"-"`
	Status                    string `json:"-"`
	RequestHash               string `json:"requestHash"`
	Phase                     string `json:"phase"`
	AccountID                 string `json:"accountId"`
	OwnerUserID               string `json:"ownerUserId"`
	WorkspaceID               string `json:"workspaceId"`
	Name                      string `json:"name"`
	PackageID                 string `json:"packageId"`
	StorageGB                 int    `json:"storageGb"`
	PricingVersion            string `json:"pricingVersion"`
	TotalMonthlyPriceCNYCents int64  `json:"totalMonthlyPriceCnyCents"`
	TotalChargeUSDMicros      int64  `json:"totalChargeUsdMicros"`
	ComputeID                 string `json:"computeAllocationId"`
	ComputeBillingOperationID string `json:"computeBillingOperationId"`
	StorageID                 string `json:"storageId"`
	StorageBillingOperationID string `json:"storageBillingOperationId"`
	AttachmentID              string `json:"attachmentId,omitempty"`
	AttachmentOperationID     string `json:"attachmentOperationId"`
	WorkspaceOperationID      string `json:"workspaceOperationId"`
	RuntimeServiceName        string `json:"runtimeServiceName,omitempty"`
	URL                       string `json:"url,omitempty"`
	ReceiptID                 string `json:"receiptId,omitempty"`
	ErrorCode                 string `json:"errorCode,omitempty"`
}

func encodeWorkspaceLaunchOperation(operation workspaceLaunchOperation) string {
	payload, _ := json.Marshal(operation)
	return string(payload)
}

func decodeWorkspaceLaunchOperation(row map[string]any) (workspaceLaunchOperation, error) {
	var operation workspaceLaunchOperation
	if err := json.Unmarshal([]byte(stringValue(row["result"])), &operation); err != nil {
		return workspaceLaunchOperation{}, errors.New("invalid_workspace_launch_operation")
	}
	operation.ID = firstNonEmpty(stringValue(row["operationId"]), stringValue(row["id"]))
	operation.Status = stringValue(row["status"])
	if operation.ID == "" || operation.Status == "" || operation.RequestHash == "" || operation.AccountID == "" || operation.WorkspaceID == "" {
		return workspaceLaunchOperation{}, errors.New("invalid_workspace_launch_operation")
	}
	for field, want := range map[string]string{
		"accountId": operation.AccountID, "workspaceId": operation.WorkspaceID, "resourceId": operation.WorkspaceID,
		"resourceKind": "workspace_launch", "action": "workspace.launch",
	} {
		if got := stringValue(row[field]); got != "" && got != want {
			return workspaceLaunchOperation{}, errors.New("invalid_workspace_launch_operation")
		}
	}
	return operation, nil
}

func workspaceLaunchOperationRow(operation workspaceLaunchOperation) map[string]any {
	return map[string]any{
		"id": operation.ID, "operationId": operation.ID, "accountId": operation.AccountID, "workspaceId": operation.WorkspaceID,
		"resourceId": operation.WorkspaceID, "resourceKind": "workspace_launch", "action": "workspace.launch", "status": operation.Status,
		"result": encodeWorkspaceLaunchOperation(operation), "computeAllocationId": operation.ComputeID, "storageId": operation.StorageID,
		"attachmentId": operation.AttachmentID, "runtimeServiceName": operation.RuntimeServiceName,
	}
}

func workspaceLaunchResponse(row map[string]any) (map[string]any, error) {
	operation, err := decodeWorkspaceLaunchOperation(row)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"operationId": operation.ID, "status": operation.Status, "phase": operation.Phase,
		"accountId": operation.AccountID, "workspaceId": operation.WorkspaceID, "name": operation.Name,
		"packageId": operation.PackageID, "storageGb": operation.StorageGB, "pricingVersion": operation.PricingVersion,
		"totalMonthlyPriceCnyCents": operation.TotalMonthlyPriceCNYCents, "totalChargeUsdMicros": operation.TotalChargeUSDMicros,
		"computeAllocationId": operation.ComputeID, "storageId": operation.StorageID, "attachmentId": operation.AttachmentID,
		"runtimeServiceName": operation.RuntimeServiceName, "url": operation.URL, "receiptId": operation.ReceiptID,
		"errorCode": operation.ErrorCode, "createdAt": row["createdAt"], "updatedAt": row["updatedAt"],
	}, nil
}
