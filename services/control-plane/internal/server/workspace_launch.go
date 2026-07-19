package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/controlplane"
	"opl-cloud/services/control-plane/internal/domain"
)

var (
	errInvalidWorkspaceLaunchOperation = errors.New("invalid_workspace_launch_operation")
	errWorkspaceLaunchInProgress       = errors.New("workspace_launch_in_progress")
	errWorkspaceLaunchCASConflict      = errors.New("workspace_launch_cas_conflict")
)

const (
	workspaceLaunchAction        = "workspace.launch.v2"
	workspaceLaunchSchemaVersion = 2
)

func isWorkspaceLaunchAction(action string) bool {
	return action == workspaceLaunchAction || action == "workspace.launch"
}

type workspaceLaunchOperation struct {
	ID                         string         `json:"-"`
	Status                     string         `json:"-"`
	CreatedAt                  string         `json:"-"`
	PersistedResult            string         `json:"-"`
	SchemaVersion              int            `json:"schemaVersion"`
	RequestHash                string         `json:"requestHash"`
	Phase                      string         `json:"phase"`
	AccountID                  string         `json:"accountId"`
	OwnerUserID                string         `json:"ownerUserId"`
	WorkspaceID                string         `json:"workspaceId"`
	Name                       string         `json:"name"`
	PackageID                  string         `json:"packageId"`
	StorageGB                  int            `json:"sizeGb"`
	AutoRenew                  bool           `json:"autoRenew"`
	PriceVersion               string         `json:"priceVersion"`
	PricingVersion             string         `json:"pricingVersion,omitempty"`
	TotalMonthlyPriceCNYCents  int64          `json:"totalMonthlyPriceCnyCents,omitempty"`
	TotalChargeUSDMicros       int64          `json:"totalChargeUsdMicros"`
	ComputeID                  string         `json:"computeAllocationId"`
	ComputeBillingOperationID  string         `json:"computeBillingOperationId"`
	StorageID                  string         `json:"storageId"`
	StorageBillingOperationID  string         `json:"storageBillingOperationId"`
	AttachmentID               string         `json:"attachmentId,omitempty"`
	AttachmentOperationID      string         `json:"attachmentOperationId"`
	WorkspaceOperationID       string         `json:"workspaceOperationId"`
	WorkspaceAPIKeyID          int64          `json:"workspaceApiKeyId"`
	RedeemCode                 string         `json:"sub2apiRedeemCode"`
	ChargeAttempted            bool           `json:"chargeAttempted,omitempty"`
	ChargeConfirmation         map[string]any `json:"chargeConfirmation,omitempty"`
	PreChargeBalanceUSDMicros  int64          `json:"preChargeBalanceUsdMicros,omitempty"`
	PostChargeBalanceUSDMicros int64          `json:"postChargeBalanceUsdMicros,omitempty"`
	PostChargeBalanceKnown     bool           `json:"postChargeBalanceKnown,omitempty"`
	LeaseToken                 string         `json:"leaseToken,omitempty"`
	LeaseExpiresAt             string         `json:"leaseExpiresAt,omitempty"`
	RuntimeServiceName         string         `json:"runtimeServiceName,omitempty"`
	URL                        string         `json:"url,omitempty"`
	ReceiptID                  string         `json:"receiptId,omitempty"`
	ErrorCode                  string         `json:"errorCode,omitempty"`
}

type workspaceLaunchClaimCAS struct {
	AccountID               string
	ExpectedOperationResult string
	DesiredOperation        map[string]any
}

type workspaceLaunchPersistCAS struct {
	OperationID             string
	ExpectedOperationResult string
	DesiredOperation        map[string]any
}

func encodeWorkspaceLaunchOperation(operation workspaceLaunchOperation) string {
	payload, _ := json.Marshal(operation)
	return string(payload)
}

func newWorkspaceLaunchOperation(accountID, ownerUserID, name, packageID string, storageGB int, autoRenew bool, priceVersion string, totalChargeUSDMicros int64, key string) workspaceLaunchOperation {
	operationID := "workspace-launch-" + stableID(accountID, key)[:18]
	workspaceID := primaryWorkspaceID(accountID)
	return workspaceLaunchOperation{
		ID: operationID, Status: "debit_pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Phase: "debit_pending", SchemaVersion: workspaceLaunchSchemaVersion,
		RequestHash: stableID("workspace-launch-v2", accountID, ownerUserID, name, packageID, strconv.Itoa(storageGB), strconv.FormatBool(autoRenew), priceVersion),
		AccountID:   accountID, OwnerUserID: ownerUserID, WorkspaceID: workspaceID, Name: name, PackageID: packageID,
		StorageGB: storageGB, AutoRenew: autoRenew, PriceVersion: priceVersion, TotalChargeUSDMicros: totalChargeUSDMicros,
		ComputeID: resourceIDForMutation("ca", accountID, operationID+":compute"), ComputeBillingOperationID: "billing-" + stableID("compute", accountID, operationID)[:18],
		StorageID: resourceIDForMutation("vol", accountID, operationID+":storage"), StorageBillingOperationID: "billing-" + stableID("storage", accountID, operationID)[:18],
		AttachmentOperationID: operationID + ":attachment", WorkspaceOperationID: operationID + ":workspace",
		RedeemCode: monthlyRedeemCode(monthlyEnvironment(), operationID),
	}
}

func decodeWorkspaceLaunchOperation(row map[string]any) (workspaceLaunchOperation, error) {
	var operation workspaceLaunchOperation
	if err := json.Unmarshal([]byte(stringValue(row["result"])), &operation); err != nil {
		return workspaceLaunchOperation{}, errInvalidWorkspaceLaunchOperation
	}
	result := stringValue(row["result"])
	operation.ID = firstNonEmpty(stringValue(row["operationId"]), stringValue(row["id"]))
	operation.Status, operation.CreatedAt, operation.PersistedResult = stringValue(row["status"]), stringValue(row["createdAt"]), result
	if operation.SchemaVersion != workspaceLaunchSchemaVersion || operation.ID == "" || operation.Status == "" || operation.RequestHash == "" || operation.AccountID == "" || operation.OwnerUserID == "" ||
		operation.WorkspaceID == "" || operation.PriceVersion == "" || operation.PackageID == "" || operation.StorageGB <= 0 || operation.TotalChargeUSDMicros <= 0 ||
		operation.WorkspaceAPIKeyID <= 0 || operation.RedeemCode == "" {
		return workspaceLaunchOperation{}, errInvalidWorkspaceLaunchOperation
	}
	for field, want := range map[string]string{
		"accountId": operation.AccountID, "workspaceId": operation.WorkspaceID, "resourceId": operation.WorkspaceID,
		"resourceKind": "workspace_launch", "action": workspaceLaunchAction,
	} {
		if got := stringValue(row[field]); got != "" && got != want {
			return workspaceLaunchOperation{}, errInvalidWorkspaceLaunchOperation
		}
	}
	return operation, nil
}

func workspaceLaunchOperationRow(operation workspaceLaunchOperation) map[string]any {
	return map[string]any{
		"id": operation.ID, "operationId": operation.ID, "accountId": operation.AccountID, "workspaceId": operation.WorkspaceID,
		"resourceId": operation.WorkspaceID, "resourceKind": "workspace_launch", "action": workspaceLaunchAction, "status": operation.Status,
		"result": encodeWorkspaceLaunchOperation(operation), "computeAllocationId": operation.ComputeID, "storageId": operation.StorageID,
		"attachmentId": operation.AttachmentID, "runtimeServiceName": operation.RuntimeServiceName, "createdAt": operation.CreatedAt,
		"workspaceApiKeyId": operation.WorkspaceAPIKeyID,
	}
}

func workspaceLaunchClaimIdentityMatches(current, desired map[string]any) bool {
	existing, existingErr := decodeWorkspaceLaunchOperation(current)
	next, nextErr := decodeWorkspaceLaunchOperation(desired)
	return existingErr == nil && nextErr == nil && existing.ID == next.ID && existing.AccountID == next.AccountID &&
		existing.WorkspaceID == next.WorkspaceID && existing.RequestHash == next.RequestHash
}

func workspaceLaunchResponse(row map[string]any) (map[string]any, error) {
	operation, err := decodeWorkspaceLaunchOperation(row)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"operationId": operation.ID, "status": operation.Status, "phase": operation.Phase,
		"accountId": operation.AccountID, "workspaceId": operation.WorkspaceID, "name": operation.Name,
		"packageId": operation.PackageID, "sizeGb": operation.StorageGB, "autoRenew": operation.AutoRenew, "priceVersion": operation.PriceVersion,
		"currency": pricingCurrency, "totalChargeUsdMicros": operation.TotalChargeUSDMicros,
		"computeAllocationId": operation.ComputeID, "storageId": operation.StorageID, "attachmentId": operation.AttachmentID,
		"workspaceApiKeyId":  strconv.FormatInt(operation.WorkspaceAPIKeyID, 10),
		"runtimeServiceName": operation.RuntimeServiceName, "url": operation.URL, "receiptId": operation.ReceiptID,
		"errorCode": operation.ErrorCode, "createdAt": row["createdAt"], "updatedAt": row["updatedAt"],
	}, nil
}

func (app *controlPlaneServer) runWorkspaceLaunchesOnce(ctx context.Context, service *controlplane.Service) error {
	rows, err := app.tables.ListRuntimeOperations(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		if stringValue(row["action"]) != workspaceLaunchAction {
			continue
		}
		operation, err := decodeWorkspaceLaunchOperation(row)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if terminalWorkspaceLaunchStatus(operation.Status) || operation.Phase == "debited" || operation.Status == "manual_review" {
			continue
		}
		if err := app.runWorkspaceLaunch(ctx, service, stringValue(row["id"])); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (app *controlPlaneServer) runWorkspaceLaunch(ctx context.Context, service *controlplane.Service, operationID string) error {
	operation, ok, err := app.workspaceLaunchOperation(ctx, operationID)
	if err != nil || !ok || terminalWorkspaceLaunchStatus(operation.Status) || operation.Phase == "debited" {
		return err
	}
	unlock := app.lockResource("workspace-launch", operation.AccountID)
	defer unlock()
	operation, ok, err = app.workspaceLaunchOperation(ctx, operationID)
	if err != nil || !ok || terminalWorkspaceLaunchStatus(operation.Status) || operation.Phase == "debited" {
		return err
	}
	unlockAccount := app.lockResource("account", operation.AccountID)
	defer unlockAccount()
	if operation.LeaseExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, operation.LeaseExpiresAt)
		if err != nil {
			return app.manualReviewWorkspaceLaunchDebit(ctx, &operation, "workspace_launch_lease_invalid")
		}
		if expiresAt.After(time.Now().UTC()) {
			return nil
		}
	}
	operation.LeaseToken = stableID(operation.ID, operation.PersistedResult, time.Now().UTC().Format(time.RFC3339Nano))
	operation.LeaseExpiresAt = time.Now().UTC().Add(workspaceRenewalLeaseDuration).Format(time.RFC3339Nano)
	desired := workspaceLaunchOperationRow(operation)
	if err := app.tables.ClaimWorkspaceLaunch(ctx, workspaceLaunchClaimCAS{
		AccountID: operation.AccountID, ExpectedOperationResult: operation.PersistedResult, DesiredOperation: desired,
	}); errors.Is(err, errWorkspaceLaunchCASConflict) {
		return nil
	} else if err != nil {
		return err
	}
	operation.PersistedResult = stringValue(desired["result"])

	owner, err := app.findUserByID(ctx, operation.OwnerUserID)
	if err != nil {
		return app.retryWorkspaceLaunchDebit(ctx, &operation, "workspace_launch_owner_state_unavailable", err)
	}
	ownerActive := owner != nil && stringValue(owner["accountId"]) == operation.AccountID
	if ownerActive {
		ownerActive, err = app.hasActiveCustomerMembership(ctx, owner)
		if err != nil {
			return app.retryWorkspaceLaunchDebit(ctx, &operation, "workspace_launch_owner_state_unavailable", err)
		}
	}
	if !ownerActive {
		return app.manualReviewWorkspaceLaunchDebit(ctx, &operation, "workspace_launch_owner_identity_mismatch")
	}
	return app.debitWorkspaceLaunch(ctx, service, &operation)
}

func (app *controlPlaneServer) runLegacyWorkspaceLaunch(ctx context.Context, service *controlplane.Service, operationID string) error {
	operation, ok, err := app.workspaceLaunchOperation(ctx, operationID)
	if err != nil || !ok || terminalWorkspaceLaunchStatus(operation.Status) {
		return err
	}
	unlock := app.lockResource("workspace-launch", operation.AccountID)
	defer unlock()
	operation, ok, err = app.workspaceLaunchOperation(ctx, operationID)
	if err != nil || !ok || terminalWorkspaceLaunchStatus(operation.Status) {
		return err
	}
	unlockAccount := app.lockResource("account", operation.AccountID)
	defer unlockAccount()
	owner, err := app.findUserByID(ctx, operation.OwnerUserID)
	if err != nil {
		return err
	}
	ownerActive := owner != nil && stringValue(owner["accountId"]) == operation.AccountID
	if ownerActive {
		ownerActive, err = app.hasActiveCustomerMembership(ctx, owner)
		if err != nil {
			return err
		}
	}
	if !ownerActive {
		return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_owner_identity_mismatch")
	}
	if operation.Status == "manual_review" {
		if app.workspaceLaunchPriceSnapshotUnavailable(operation) {
			return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_price_snapshot_unavailable")
		}
		resume, err := app.reconcileWorkspaceLaunchChildReview(ctx, &operation)
		if err != nil || !resume {
			return err
		}
	}

	for range 6 {
		if app.workspaceLaunchPriceSnapshotUnavailable(operation) {
			return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_price_snapshot_unavailable")
		}
		switch operation.Phase {
		case "compute":
			row, err := app.purchaseWorkspaceLaunchResource(ctx, service, monthlyPurchaseInput{
				ResourceType: "compute", ResourceID: operation.ComputeID, BillingOperationID: operation.ComputeBillingOperationID,
				AccountID: operation.AccountID, OwnerUserID: operation.OwnerUserID, WorkspaceID: operation.WorkspaceID,
				Name: operation.Name, PackageID: operation.PackageID, Zone: monthlyComputeLaunchZone(), Environment: monthlyEnvironment(), AutoRenew: &operation.AutoRenew,
			})
			if err != nil {
				return app.failWorkspaceLaunchPurchase(ctx, operation, row, err)
			}
			if stringValue(row["billingStatus"]) != "active" {
				return app.waitWorkspaceLaunch(ctx, operation)
			}
			operation.Phase, operation.Status, operation.ErrorCode = "storage", "preparing", ""
			if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
				return err
			}

		case "storage":
			compute, ok := app.getCompute(operation.ComputeID)
			if !ok {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_compute_missing")
			}
			zone := firstNonEmpty(stringValue(compute["zone"]), providerDataValue(compute, "zone"))
			if zone == "" {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_compute_zone_unavailable")
			}
			row, err := app.purchaseWorkspaceLaunchResource(ctx, service, monthlyPurchaseInput{
				ResourceType: "storage", ResourceID: operation.StorageID, BillingOperationID: operation.StorageBillingOperationID,
				AccountID: operation.AccountID, OwnerUserID: operation.OwnerUserID, WorkspaceID: operation.WorkspaceID,
				Name: operation.Name, PackageID: operation.PackageID, SizeGB: operation.StorageGB, ComputeID: operation.ComputeID,
				Zone: zone, Environment: monthlyEnvironment(), AutoRenew: &operation.AutoRenew,
			})
			if err != nil {
				return app.failWorkspaceLaunchPurchase(ctx, operation, row, err)
			}
			if stringValue(row["billingStatus"]) != "active" {
				return app.waitWorkspaceLaunch(ctx, operation)
			}
			operation.Phase, operation.Status, operation.ErrorCode = "attachment", "preparing", ""
			if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
				return err
			}

		case "attachment":
			if attachment, ok := app.workspaceLaunchAttachment(operation); ok {
				operation.AttachmentID = stringValue(attachment["id"])
			} else {
				created, err := service.CreateStorageAttachment(ctx, controlplane.StorageAttachmentInput{
					WorkspaceID: operation.WorkspaceID, ComputeID: operation.ComputeID, VolumeID: operation.StorageID,
				}, operation.AttachmentOperationID)
				if err != nil {
					return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_attachment_retryable")
				}
				if created.ID == "" || created.WorkspaceID != operation.WorkspaceID || created.ComputeID != operation.ComputeID || created.VolumeID != operation.StorageID {
					return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_attachment_identity_mismatch")
				}
				body := attachmentResponse(structToMap(created), map[string]any{
					"computeAllocationId": operation.ComputeID, "storageId": operation.StorageID, "workspaceId": operation.WorkspaceID,
				})
				body["accountId"], body["packageId"], body["operationId"] = operation.AccountID, operation.PackageID, operation.AttachmentOperationID
				if err := app.saveAttachmentFact(body, body); err != nil {
					return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_attachment_persist_retryable")
				}
				operation.AttachmentID = created.ID
			}
			operation.Phase, operation.Status, operation.ErrorCode = "workspace", "preparing", ""
			if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
				return err
			}

		case "workspace":
			billingState, reviewCode := app.workspaceLaunchBillingState(ctx, operation)
			if reviewCode != "" {
				return app.manualReviewWorkspaceLaunch(ctx, operation, reviewCode)
			}
			if workspace, ok := app.getWorkspace(operation.WorkspaceID); ok {
				if !workspaceMatchesLaunch(workspace, operation) {
					return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_projection_identity_mismatch")
				}
				if !workspaceBillingStateMatchesLaunch(workspace, billingState) {
					return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_billing_state_mismatch")
				}
				if stringValue(workspace["runtimeId"]) != "" {
					operation.RuntimeServiceName = firstNonEmpty(stringValue(workspace["runtimeServiceName"]), stringValue(nested(workspace, "runtime", "serviceName")))
					operation.URL = stringValue(workspace["url"])
					operation.Phase, operation.Status, operation.ErrorCode = "receipt", "preparing", ""
					if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
						return err
					}
					continue
				}
			}
			sub2APIUserID, err := app.sub2APIUserID(ctx, operation.AccountID)
			if err != nil {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_account_mapping_unavailable")
			}
			workspace, err := service.PrepareWorkspace(ctx, controlplane.CreateWorkspaceInput{
				WorkspaceID: operation.WorkspaceID, AccountID: operation.AccountID, Sub2APIUserID: sub2APIUserID,
				WorkspaceAPIKeyID: operation.WorkspaceAPIKeyID,
				OwnerID:           operation.OwnerUserID, Name: operation.Name, PackageID: operation.PackageID,
				AttachmentID: operation.AttachmentID, ComputeID: operation.ComputeID, VolumeID: operation.StorageID,
			}, operation.WorkspaceOperationID)
			if err != nil {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_runtime_retryable")
			}
			if !workspaceProjectionMatchesLaunch(workspace, operation) {
				return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_runtime_identity_mismatch")
			}
			workspaceRow := workspaceProjectionRow(workspace)
			for key, value := range billingState {
				workspaceRow[key] = value
			}
			workspaceRow, err = app.tables.ActivateWorkspace(ctx, workspaceRow)
			if errors.Is(err, errWorkspaceActivationConflict) {
				return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_activation_conflict")
			}
			if err != nil {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_projection_persist_retryable")
			}
			operation.RuntimeServiceName, operation.URL = workspace.RuntimeServiceName, workspace.URL
			operation.Phase, operation.Status, operation.ErrorCode = "receipt", "preparing", ""
			if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
				return err
			}

		case "receipt":
			workspace, ok := app.getWorkspace(operation.WorkspaceID)
			if !ok || !workspaceMatchesLaunch(workspace, operation) {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_projection_unavailable")
			}
			recorded, err := service.RecordWorkspaceCreatedReceipt(ctx, domain.WorkspaceProjection{
				ID: operation.WorkspaceID, AccountID: operation.AccountID, URL: stringValue(workspace["url"]), RuntimeID: stringValue(workspace["runtimeId"]),
			}, operation.WorkspaceOperationID)
			if err != nil {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_receipt_retryable")
			}
			workspace["receiptId"] = recorded.ReceiptID
			if err := app.tables.SaveWorkspace(ctx, workspace); err != nil {
				return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_receipt_projection_retryable")
			}
			operation.ReceiptID, operation.Phase, operation.Status, operation.ErrorCode = recorded.ReceiptID, "complete", "succeeded", ""
			return app.saveWorkspaceLaunchOperation(ctx, operation)

		case "complete":
			return nil
		default:
			return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_phase_invalid")
		}
	}
	return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_transition_limit")
}

type workspaceBillingChildIdentity struct {
	AccountID, OwnerUserID, WorkspaceID, PackageID, ComputeID, StorageID string
	StorageGB                                                            int64
}

func workspaceBillingStateFromChildren(compute, storage map[string]any, identity workspaceBillingChildIdentity) (map[string]any, string) {
	if stringValue(compute["id"]) != identity.ComputeID || stringValue(storage["id"]) != identity.StorageID ||
		stringValue(compute["accountId"]) != identity.AccountID || stringValue(storage["accountId"]) != identity.AccountID ||
		stringValue(compute["workspaceId"]) != identity.WorkspaceID || stringValue(storage["workspaceId"]) != identity.WorkspaceID ||
		stringValue(compute["ownerUserId"]) != identity.OwnerUserID || stringValue(storage["ownerUserId"]) != identity.OwnerUserID ||
		stringValue(compute["packageId"]) != identity.PackageID || stringValue(storage["packageId"]) != identity.PackageID ||
		stringValue(storage["computeAllocationId"]) != identity.ComputeID || stringValue(compute["billingStatus"]) != "active" || stringValue(storage["billingStatus"]) != "active" {
		return nil, "workspace_launch_billing_identity_mismatch"
	}
	storageGB, validStorageGB := requiredPositiveInteger(storage, "sizeGb")
	if !validStorageGB || storageGB != identity.StorageGB {
		return nil, "workspace_launch_billing_identity_mismatch"
	}
	compute = cloneMap(compute)
	storage = cloneMap(storage)
	if !monthlyPriceSnapshotAvailable(compute) || !monthlyPriceSnapshotAvailable(storage) ||
		stringValue(compute["priceVersion"]) != pricingCatalogVersion || stringValue(storage["priceVersion"]) != pricingCatalogVersion ||
		compute["currency"] != pricingCurrency || storage["currency"] != pricingCurrency {
		return nil, "workspace_launch_billing_price_mismatch"
	}
	quote, err := workspacePricingPreview(defaultPricingCatalog(), map[string]any{"packageId": identity.PackageID, "sizeGb": identity.StorageGB})
	if err != nil {
		return nil, "workspace_launch_billing_price_mismatch"
	}
	computePrice, validComputePrice := requiredPositiveInteger(compute, "chargeUsdMicros")
	storagePrice, validStoragePrice := requiredPositiveInteger(storage, "chargeUsdMicros")
	expectedCompute, expectedComputeOK := requiredPositiveInteger(mapField(quote, "compute"), "chargeUsdMicros")
	expectedStorage, expectedStorageOK := requiredPositiveInteger(mapField(quote, "storage"), "chargeUsdMicros")
	total, validTotal := checkedAddInt64(computePrice, storagePrice)
	if !validComputePrice || !validStoragePrice || !expectedComputeOK || !expectedStorageOK || !validTotal ||
		computePrice != expectedCompute || storagePrice != expectedStorage || stringValue(quote["priceVersion"]) != pricingCatalogVersion {
		return nil, "workspace_launch_billing_price_mismatch"
	}
	computeStart, computeStartErr := time.Parse(time.RFC3339, stringValue(compute["periodStart"]))
	storageStart, storageStartErr := time.Parse(time.RFC3339, stringValue(storage["periodStart"]))
	computePaid, computePaidErr := time.Parse(time.RFC3339, stringValue(compute["paidThrough"]))
	storagePaid, storagePaidErr := time.Parse(time.RFC3339, stringValue(storage["paidThrough"]))
	computeAnchor, computeAnchorOK := requiredPositiveInteger(compute, "billingAnchorDay")
	storageAnchor, storageAnchorOK := requiredPositiveInteger(storage, "billingAnchorDay")
	if computeStartErr != nil || storageStartErr != nil || computePaidErr != nil || storagePaidErr != nil || !computeAnchorOK || !storageAnchorOK || computeAnchor > 31 || computeAnchor != storageAnchor {
		return nil, "workspace_launch_billing_period_mismatch"
	}
	periodStart := computeStart
	if storageStart.After(periodStart) {
		periodStart = storageStart
	}
	paidThrough := computePaid
	if storagePaid.Before(paidThrough) {
		paidThrough = storagePaid
	}
	if !paidThrough.After(periodStart) {
		return nil, "workspace_launch_billing_period_mismatch"
	}
	computeDeadline, computeDeadlineErr := monthlyProviderDeadline(compute)
	storageDeadline, storageDeadlineErr := monthlyProviderDeadline(storage)
	if computeDeadlineErr != nil || storageDeadlineErr != nil || computeDeadline.Before(paidThrough) || storageDeadline.Before(paidThrough) ||
		!monthlyPurchaseReadbackConfirmed("compute", compute, compute) || !monthlyPurchaseReadbackConfirmed("storage", storage, storage) {
		return nil, "workspace_launch_provider_deadline_invalid"
	}
	state := map[string]any{
		"ownerUserId": identity.OwnerUserID, "currentComputeAllocationId": identity.ComputeID,
		"autoRenew": false, "authorizedBy": "", "authorizedAt": "",
		"packageId": identity.PackageID, "storageGb": identity.StorageGB,
		"priceVersion": pricingCatalogVersion, "currency": pricingCurrency, "billingUnit": pricingBillingUnit,
		"computeUsdMicros": computePrice, "storageUsdMicros": storagePrice, "totalUsdMicros": total,
		"periodStart": periodStart.UTC().Format(time.RFC3339Nano), "paidThrough": paidThrough.UTC().Format(time.RFC3339Nano),
		"nextRenewalAt": paidThrough.UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano), "billingAnchorDay": computeAnchor,
		"renewalStatus": "active", "computeAllocationId": identity.ComputeID, "storageId": identity.StorageID,
	}
	if err := validateWorkspaceBillingState(state); err != nil {
		return nil, "workspace_launch_billing_state_invalid"
	}
	return state, ""
}

func (app *controlPlaneServer) workspaceLaunchBillingState(ctx context.Context, operation workspaceLaunchOperation) (map[string]any, string) {
	compute, computeOK := app.getCompute(operation.ComputeID)
	storage, storageOK := app.getStorage(operation.StorageID)
	if !computeOK || !storageOK {
		return nil, "workspace_launch_billing_identity_mismatch"
	}
	state, code := workspaceBillingStateFromChildren(compute, storage, workspaceBillingChildIdentity{
		AccountID: operation.AccountID, OwnerUserID: operation.OwnerUserID, WorkspaceID: operation.WorkspaceID,
		PackageID: operation.PackageID, ComputeID: operation.ComputeID, StorageID: operation.StorageID, StorageGB: int64(operation.StorageGB),
	})
	if code != "" {
		return nil, code
	}
	if stringValue(state["priceVersion"]) != operation.PriceVersion || state["totalUsdMicros"] != operation.TotalChargeUSDMicros {
		return nil, "workspace_launch_billing_price_mismatch"
	}
	autoRenew, authorizedBy, authorizedAt := operation.AutoRenew, "", ""
	owner, ownerErr := app.findUserByID(ctx, operation.OwnerUserID)
	if ownerErr != nil {
		return nil, "workspace_launch_owner_state_unavailable"
	}
	if owner == nil || stringValue(owner["accountId"]) != operation.AccountID || stringValue(owner["status"]) != "active" || stringValue(owner["role"]) != "owner" {
		autoRenew = false
		if owner != nil && stringValue(owner["accountId"]) == operation.AccountID && stringValue(owner["role"]) == "owner" {
			if err := app.tables.ApplyUserLifecycle(ctx, owner); err != nil {
				return nil, "workspace_launch_owner_state_unavailable"
			}
		} else {
			computeErr := app.tables.SetResourceAutoRenew(ctx, "compute", operation.ComputeID, operation.AccountID, false)
			storageErr := app.tables.SetResourceAutoRenew(ctx, "storage", operation.StorageID, operation.AccountID, false)
			if computeErr != nil || storageErr != nil {
				return nil, "workspace_launch_owner_state_unavailable"
			}
		}
	} else if autoRenew {
		createdAt, err := time.Parse(time.RFC3339, operation.CreatedAt)
		if err != nil {
			return nil, "workspace_launch_authorization_invalid"
		}
		authorizedBy, authorizedAt = operation.OwnerUserID, createdAt.UTC().Format(time.RFC3339Nano)
	}
	state["autoRenew"], state["authorizedBy"], state["authorizedAt"] = autoRenew, authorizedBy, authorizedAt
	if err := validateWorkspaceBillingState(state); err != nil {
		return nil, "workspace_launch_billing_state_invalid"
	}
	return state, ""
}

func workspaceBillingStateMatchesLaunch(workspace, expected map[string]any) bool {
	currentJSON, currentErr := encodeWorkspaceBillingState(workspace)
	expectedJSON, expectedErr := encodeWorkspaceBillingState(expected)
	return currentErr == nil && expectedErr == nil && currentJSON == expectedJSON
}

func terminalWorkspaceLaunchStatus(status string) bool {
	return status == "succeeded" || status == "refunded" || status == "failed"
}

func workspaceLaunchChildReview(operation workspaceLaunchOperation) bool {
	return (operation.Phase == "compute" || operation.Phase == "storage") &&
		operation.ErrorCode == "workspace_launch_"+operation.Phase+"_manual_review"
}

func (app *controlPlaneServer) workspaceLaunchPriceSnapshotUnavailable(operation workspaceLaunchOperation) bool {
	if operation.Phase != "compute" && operation.Phase != "storage" || operation.PriceVersion == pricingCatalogVersion {
		return false
	}
	resourceID, billingOperationID := operation.ComputeID, operation.ComputeBillingOperationID
	if operation.Phase == "storage" {
		resourceID, billingOperationID = operation.StorageID, operation.StorageBillingOperationID
	}
	child, ok := app.monthlyResource(operation.Phase, resourceID)
	return !ok || stringValue(child["accountId"]) != operation.AccountID || stringValue(child["billingOperationId"]) != billingOperationID ||
		firstNonEmpty(stringValue(child["priceVersion"]), stringValue(child["pricingVersion"])) != operation.PriceVersion || !monthlyPriceSnapshotAvailable(child)
}

func (app *controlPlaneServer) reconcileWorkspaceLaunchChildReview(ctx context.Context, operation *workspaceLaunchOperation) (bool, error) {
	if !workspaceLaunchChildReview(*operation) {
		return false, nil
	}
	resourceID, billingOperationID := operation.ComputeID, operation.ComputeBillingOperationID
	if operation.Phase == "storage" {
		resourceID, billingOperationID = operation.StorageID, operation.StorageBillingOperationID
	}
	child, ok := app.monthlyResource(operation.Phase, resourceID)
	if !ok || stringValue(child["accountId"]) != operation.AccountID || stringValue(child["billingOperationId"]) != billingOperationID {
		return false, nil
	}
	switch stringValue(child["billingStatus"]) {
	case "active":
		operation.Status, operation.ErrorCode = "preparing", ""
		if err := app.saveWorkspaceLaunchOperation(ctx, *operation); err != nil {
			return false, err
		}
		return true, nil
	case "refunded", "failed":
		operation.Status = stringValue(child["billingStatus"])
		operation.ErrorCode = "workspace_launch_" + operation.Phase + "_" + operation.Status
		return false, app.saveWorkspaceLaunchOperation(ctx, *operation)
	default:
		return false, nil
	}
}

func (app *controlPlaneServer) purchaseWorkspaceLaunchResource(ctx context.Context, service *controlplane.Service, input monthlyPurchaseInput) (map[string]any, error) {
	unlock := app.lockResource(input.ResourceType, input.ResourceID)
	defer unlock()
	return app.purchaseMonthlyResource(ctx, service, input)
}

func (app *controlPlaneServer) workspaceLaunchOperation(ctx context.Context, operationID string) (workspaceLaunchOperation, bool, error) {
	rows, err := app.tables.ListRuntimeOperations(ctx)
	if err != nil {
		return workspaceLaunchOperation{}, false, err
	}
	for _, row := range rows {
		if stringValue(row["id"]) != operationID || stringValue(row["action"]) != workspaceLaunchAction {
			continue
		}
		operation, err := decodeWorkspaceLaunchOperation(row)
		return operation, err == nil, err
	}
	return workspaceLaunchOperation{}, false, nil
}

func releaseWorkspaceLaunchLease(operation *workspaceLaunchOperation) {
	operation.LeaseToken, operation.LeaseExpiresAt = "", ""
}

func (app *controlPlaneServer) persistWorkspaceLaunch(ctx context.Context, operation *workspaceLaunchOperation) error {
	desired := workspaceLaunchOperationRow(*operation)
	if err := app.tables.PersistWorkspaceLaunch(ctx, workspaceLaunchPersistCAS{
		OperationID: operation.ID, ExpectedOperationResult: operation.PersistedResult, DesiredOperation: desired,
	}); err != nil {
		return err
	}
	operation.PersistedResult = stringValue(desired["result"])
	return nil
}

func (app *controlPlaneServer) retryWorkspaceLaunchDebit(ctx context.Context, operation *workspaceLaunchOperation, code string, cause error) error {
	if cause == nil {
		cause = errors.New(code)
	}
	operation.Status, operation.Phase, operation.ErrorCode = "unknown", "debit_pending", code
	releaseWorkspaceLaunchLease(operation)
	return errors.Join(cause, app.persistWorkspaceLaunch(ctx, operation))
}

func (app *controlPlaneServer) manualReviewWorkspaceLaunchDebit(ctx context.Context, operation *workspaceLaunchOperation, code string) error {
	operation.Status, operation.ErrorCode = "manual_review", code
	releaseWorkspaceLaunchLease(operation)
	return errors.Join(errors.New(code), app.persistWorkspaceLaunch(ctx, operation))
}

func (app *controlPlaneServer) debitWorkspaceLaunch(ctx context.Context, service *controlplane.Service, operation *workspaceLaunchOperation) error {
	userID, err := app.sub2APIUserID(ctx, operation.AccountID)
	if err != nil {
		return app.retryWorkspaceLaunchDebit(ctx, operation, errMonthlyAccountUnmapped.Error(), err)
	}
	key, err := service.Sub2APIWorkspaceKeyByID(ctx, userID, operation.WorkspaceAPIKeyID)
	if err != nil || key.ID != operation.WorkspaceAPIKeyID || key.UserID != userID || key.Name != "opl-workspace" || key.Status != "active" {
		return app.retryWorkspaceLaunchDebit(ctx, operation, "gateway_key_unavailable", err)
	}
	if operation.ChargeConfirmation == nil {
		var charge clients.Sub2APICharge
		if operation.ChargeAttempted || operation.Status == "unknown" {
			history, historyErr := service.Sub2APIBalanceHistory(ctx, userID)
			row := map[string]any{"sub2apiRedeemCode": operation.RedeemCode, "chargeUsdMicros": operation.TotalChargeUSDMicros}
			switch code := sub2APIReconciliationCode(row, userID, history); {
			case historyErr != nil || code == "sub2api_charge_missing":
				charge, err = service.ChargeSub2API(ctx, clients.Sub2APIChargeInput{
					UserID: userID, Code: operation.RedeemCode, ChargeUSDMicros: operation.TotalChargeUSDMicros, Notes: "OPL Workspace launch " + operation.WorkspaceID,
				})
			case code != "":
				return app.manualReviewWorkspaceLaunchDebit(ctx, operation, code)
			default:
				charge = clients.Sub2APICharge{Code: operation.RedeemCode, UserID: userID, ChargeUSDMicros: operation.TotalChargeUSDMicros, Status: "used"}
			}
		} else {
			balance, balanceErr := service.Sub2APIBalance(ctx, userID)
			if balanceErr != nil {
				return app.retryWorkspaceLaunchDebit(ctx, operation, "sub2api_balance_unavailable", balanceErr)
			}
			if balance.USDMicros < operation.TotalChargeUSDMicros {
				operation.Status, operation.Phase, operation.ErrorCode = "insufficient", "debit_pending", errMonthlyInsufficientBalance.Error()
				releaseWorkspaceLaunchLease(operation)
				if err := app.persistWorkspaceLaunch(ctx, operation); err != nil {
					return err
				}
				return errMonthlyInsufficientBalance
			}
			operation.PreChargeBalanceUSDMicros, operation.ChargeAttempted = balance.USDMicros, true
			if err := app.persistWorkspaceLaunch(ctx, operation); err != nil {
				return err
			}
			charge, err = service.ChargeSub2API(ctx, clients.Sub2APIChargeInput{
				UserID: userID, Code: operation.RedeemCode, ChargeUSDMicros: operation.TotalChargeUSDMicros, Notes: "OPL Workspace launch " + operation.WorkspaceID,
			})
		}
		if err != nil {
			if errors.Is(err, clients.ErrSub2APIChargeUnknown) {
				return app.retryWorkspaceLaunchDebit(ctx, operation, "sub2api_charge_unconfirmed", err)
			}
			if errors.Is(err, errMonthlyInsufficientBalance) {
				operation.Status, operation.Phase, operation.ErrorCode = "insufficient", "debit_pending", errMonthlyInsufficientBalance.Error()
				releaseWorkspaceLaunchLease(operation)
				return errors.Join(err, app.persistWorkspaceLaunch(ctx, operation))
			}
			return app.manualReviewWorkspaceLaunchDebit(ctx, operation, "sub2api_charge_unconfirmed")
		}
		confirmation := map[string]any{"code": charge.Code, "userId": charge.UserID, "chargeUsdMicros": charge.ChargeUSDMicros, "status": charge.Status}
		if !monthlyChargeConfirmationMatches(confirmation, operation.RedeemCode, userID, operation.TotalChargeUSDMicros) {
			return app.manualReviewWorkspaceLaunchDebit(ctx, operation, "sub2api_charge_confirmation_invalid")
		}
		operation.ChargeConfirmation, operation.ErrorCode = confirmation, ""
		if err := app.persistWorkspaceLaunch(ctx, operation); err != nil {
			return err
		}
	}
	history, historyErr := service.Sub2APIBalanceHistory(ctx, userID)
	row := map[string]any{"sub2apiRedeemCode": operation.RedeemCode, "chargeUsdMicros": operation.TotalChargeUSDMicros}
	if historyErr != nil || sub2APIReconciliationCode(row, userID, history) == "sub2api_charge_missing" {
		return app.retryWorkspaceLaunchDebit(ctx, operation, "sub2api_charge_history_unavailable", errors.Join(historyErr, clients.ErrSub2APIChargeUnknown))
	}
	if code := sub2APIReconciliationCode(row, userID, history); code != "" {
		return app.manualReviewWorkspaceLaunchDebit(ctx, operation, code)
	}
	postCharge, err := service.Sub2APIBalance(ctx, userID)
	if err != nil {
		return app.retryWorkspaceLaunchDebit(ctx, operation, "post_charge_balance_unavailable", err)
	}
	operation.PostChargeBalanceKnown, operation.PostChargeBalanceUSDMicros = true, postCharge.USDMicros
	if postCharge.USDMicros < 0 || operation.PreChargeBalanceUSDMicros > 0 && postCharge.USDMicros > operation.PreChargeBalanceUSDMicros-operation.TotalChargeUSDMicros {
		return app.manualReviewWorkspaceLaunchDebit(ctx, operation, "post_charge_balance_invalid")
	}
	operation.Status, operation.Phase, operation.ErrorCode = "debited", "debited", ""
	releaseWorkspaceLaunchLease(operation)
	return app.persistWorkspaceLaunch(ctx, operation)
}

func (app *controlPlaneServer) saveWorkspaceLaunchOperation(ctx context.Context, operation workspaceLaunchOperation) error {
	return app.tables.SaveRuntimeOperation(ctx, workspaceLaunchOperationRow(operation))
}

func (app *controlPlaneServer) waitWorkspaceLaunch(ctx context.Context, operation workspaceLaunchOperation) error {
	operation.Status, operation.ErrorCode = "waiting", ""
	return app.saveWorkspaceLaunchOperation(ctx, operation)
}

func (app *controlPlaneServer) retryWorkspaceLaunch(ctx context.Context, operation workspaceLaunchOperation, code string) error {
	operation.Status, operation.ErrorCode = "retryable", code
	if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
		return err
	}
	return errors.New(code)
}

func (app *controlPlaneServer) manualReviewWorkspaceLaunch(ctx context.Context, operation workspaceLaunchOperation, code string) error {
	operation.Status, operation.ErrorCode = "manual_review", code
	if err := app.saveWorkspaceLaunchOperation(ctx, operation); err != nil {
		return err
	}
	return errors.New(code)
}

func (app *controlPlaneServer) failWorkspaceLaunchPurchase(ctx context.Context, operation workspaceLaunchOperation, row map[string]any, cause error) error {
	status := stringValue(row["billingStatus"])
	if errors.Is(cause, errMonthlyPriceSnapshotUnavailable) {
		return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_price_snapshot_unavailable")
	}
	if status == "failed" && errors.Is(cause, errMonthlyInsufficientBalance) {
		row["billingStatus"] = "charge_pending"
		if err := app.saveMonthlyResource(ctx, operation.Phase, row); err != nil {
			return err
		}
		return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_balance_insufficient")
	}
	if status == "manual_review" || status == "failed" || errors.Is(cause, errMonthlyChargeNeedsReview) || errors.Is(cause, errMonthlyInsufficientBalance) || errors.Is(cause, errMonthlyPurchaseRefunded) || errors.Is(cause, errIdempotencyConflict) {
		return app.manualReviewWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_manual_review")
	}
	return app.retryWorkspaceLaunch(ctx, operation, "workspace_launch_"+operation.Phase+"_retryable")
}

func (app *controlPlaneServer) workspaceLaunchAttachment(operation workspaceLaunchOperation) (map[string]any, bool) {
	for _, attachment := range app.listAttachments(operation.AccountID) {
		if stringValue(attachment["operationId"]) == operation.AttachmentOperationID && attachmentMatchesLaunch(attachment, operation) {
			return attachment, true
		}
	}
	return nil, false
}

func attachmentMatchesLaunch(attachment map[string]any, operation workspaceLaunchOperation) bool {
	return stringValue(attachment["workspaceId"]) == operation.WorkspaceID &&
		firstNonEmpty(stringValue(attachment["computeAllocationId"]), stringValue(attachment["computeId"])) == operation.ComputeID &&
		firstNonEmpty(stringValue(attachment["storageId"]), stringValue(attachment["volumeId"])) == operation.StorageID
}

func workspaceMatchesLaunch(workspace map[string]any, operation workspaceLaunchOperation) bool {
	return firstNonEmpty(stringValue(workspace["accountId"]), stringValue(workspace["ownerAccountId"])) == operation.AccountID &&
		firstNonEmpty(stringValue(workspace["ownerUserId"]), stringValue(workspace["ownerId"])) == operation.OwnerUserID &&
		stringValue(workspace["packageId"]) == operation.PackageID &&
		firstNonEmpty(stringValue(workspace["computeAllocationId"]), stringValue(workspace["currentComputeAllocationId"])) == operation.ComputeID &&
		stringValue(workspace["storageId"]) == operation.StorageID && int64(numberField(workspace, "workspaceApiKeyId", 0)) == operation.WorkspaceAPIKeyID &&
		firstNonEmpty(stringValue(workspace["attachmentId"]), stringValue(workspace["currentAttachmentId"])) == operation.AttachmentID
}

func workspaceProjectionMatchesLaunch(workspace domain.WorkspaceProjection, operation workspaceLaunchOperation) bool {
	return workspace.ID == operation.WorkspaceID && workspace.AccountID == operation.AccountID && workspace.OwnerID == operation.OwnerUserID &&
		workspace.PackageID == operation.PackageID && workspace.ComputeID == operation.ComputeID && workspace.VolumeID == operation.StorageID && workspace.AttachmentID == operation.AttachmentID && workspace.WorkspaceAPIKeyID == operation.WorkspaceAPIKeyID
}
