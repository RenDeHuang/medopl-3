package server

import (
	"encoding/json"
	"net/http"

	"opl-cloud/services/control-plane/internal/controlplane"
)

func registerWorkspaceRoutes(mux *http.ServeMux, app *controlPlaneServer, service *controlplane.Service) {
	mux.HandleFunc("GET /api/workspaces", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := app.scopedAccountID(w, r, nil)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, app.state(accountID, nil)["workspaces"])
	}))
	mux.HandleFunc("POST /api/workspaces/reset-token", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		workspaceID := stringField(input, "workspaceId", "")
		before, exists := app.getWorkspace(workspaceID)
		if exists && !app.canAccessResource(r, before) {
			writeError(w, http.StatusForbidden, "account_scope_forbidden")
			return
		}
		workspace, ok, err := app.setWorkspaceAccess(workspaceID, "active")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "workspace_not_found")
			return
		}
		if err := app.appendAuditEvent(r, "workspace.reset_token", "workspace", workspaceID, stringValue(workspace["accountId"]), before, workspace, "succeeded"); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": workspace["id"], "tokenStatus": nested(workspace, "access", "tokenStatus"), "access": workspace["access"]})
	}))
	mux.HandleFunc("POST /api/workspaces/delete-token", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		workspaceID := stringField(input, "workspaceId", "")
		before, exists := app.getWorkspace(workspaceID)
		if exists && !app.canAccessResource(r, before) {
			writeError(w, http.StatusForbidden, "account_scope_forbidden")
			return
		}
		workspace, ok, err := app.setWorkspaceAccess(workspaceID, "disabled")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "workspace_not_found")
			return
		}
		if err := app.appendAuditEvent(r, "workspace.delete_token", "workspace", workspaceID, stringValue(workspace["accountId"]), before, workspace, "succeeded"); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": workspace["id"], "tokenStatus": nested(workspace, "access", "tokenStatus"), "access": workspace["access"]})
	}))
	mux.HandleFunc("POST /api/workspaces/runtime-status", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		workspaceID := stringField(input, "workspaceId", "")
		workspace, ok := app.getWorkspace(workspaceID)
		if !ok {
			writeError(w, http.StatusNotFound, "workspace_not_found")
			return
		}
		if !app.canAccessResource(r, workspace) {
			writeError(w, http.StatusForbidden, "account_scope_forbidden")
			return
		}
		runtime, err := service.WorkspaceRuntimeStatus(r.Context(), workspaceID)
		if err != nil {
			writeUpstreamError(w)
			return
		}
		writeJSON(w, http.StatusOK, workspaceRuntimeStatusResponse(runtime))
	}))
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/resume", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		workspaceID := r.PathValue("workspaceId")
		workspace, ok := app.getWorkspace(workspaceID)
		if !ok {
			writeError(w, http.StatusNotFound, "workspace_not_found")
			return
		}
		if !app.canAccessResource(r, workspace) {
			writeError(w, http.StatusForbidden, "account_scope_forbidden")
			return
		}
		key, ok := executionMutationKey(w, r)
		if !ok {
			return
		}
		requestHash := stableID(string(mustJSON(input)))
		operationID := "resume-" + stableID(workspaceID, key)[:18]
		operations, err := app.tables.ListRuntimeOperations(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "state_read_failed")
			return
		}
		for _, operation := range operations {
			if stringValue(operation["id"]) != operationID {
				continue
			}
			var result struct {
				RequestHash string         `json:"requestHash"`
				Response    map[string]any `json:"response"`
			}
			if json.Unmarshal([]byte(stringValue(operation["result"])), &result) != nil || result.Response == nil {
				writeError(w, http.StatusInternalServerError, "state_read_failed")
				return
			}
			if result.RequestHash != requestHash {
				writeError(w, http.StatusConflict, "idempotency_conflict")
				return
			}
			writeJSON(w, http.StatusOK, result.Response)
			return
		}
		state := firstNonEmpty(stringValue(workspace["state"]), stringValue(workspace["status"]))
		if state != "suspended" && state != "stopped" {
			writeError(w, http.StatusConflict, "workspace_not_suspended")
			return
		}
		accountID := firstNonEmpty(stringValue(workspace["accountId"]), stringValue(workspace["ownerAccountId"]))
		storageID := stringValue(workspace["storageId"])
		computeID := stringField(input, "computeAllocationId", "")
		attachmentID := stringField(input, "attachmentId", "")
		storage, storageOK := app.getStorage(storageID)
		compute, computeOK := app.getCompute(computeID)
		attachment, attachmentOK := app.getAttachment(attachmentID)
		if !storageOK || !computeOK || !attachmentOK {
			writeError(w, http.StatusConflict, "resume_resources_not_ready")
			return
		}
		storageStatus := stringValue(storage["status"])
		computeStatus := stringValue(compute["status"])
		attachmentStatus := stringValue(attachment["status"])
		if !app.resourceBelongsToAccount(storage, accountID) || !app.resourceBelongsToAccount(compute, accountID) || !app.resourceBelongsToAccount(attachment, accountID) ||
			stringValue(storage["workspaceId"]) != workspaceID || stringValue(compute["workspaceId"]) != workspaceID || stringValue(attachment["workspaceId"]) != workspaceID ||
			firstNonEmpty(stringValue(attachment["computeAllocationId"]), stringValue(attachment["computeId"])) != computeID || firstNonEmpty(stringValue(attachment["storageId"]), stringValue(attachment["volumeId"])) != storageID {
			writeError(w, http.StatusConflict, "resume_resource_mismatch")
			return
		}
		if (storageStatus != "ready" && storageStatus != "available") || (computeStatus != "running" && computeStatus != "ready" && computeStatus != "available" && computeStatus != "active") || (attachmentStatus != "attached" && attachmentStatus != "ready") {
			writeError(w, http.StatusConflict, "resume_resources_not_ready")
			return
		}
		resumed, err := service.ResumeWorkspace(r.Context(), controlplane.ResumeWorkspaceInput{WorkspaceID: workspaceID, AccountID: accountID, OwnerID: stringValue(workspace["ownerUserId"]), Name: stringValue(workspace["name"]), PackageID: stringValue(workspace["packageId"]), URL: stringValue(workspace["url"]), AttachmentID: attachmentID, ComputeID: computeID, VolumeID: storageID}, key)
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		before := cloneMap(workspace)
		workspace["state"], workspace["status"] = resumed.Status, resumed.Status
		workspace["computeAllocationId"], workspace["currentComputeAllocationId"] = resumed.ComputeID, resumed.ComputeID
		workspace["attachmentId"], workspace["currentAttachmentId"] = resumed.AttachmentID, resumed.AttachmentID
		workspace["runtimeId"] = resumed.RuntimeID
		workspace["runtime"] = map[string]any{"serviceName": resumed.RuntimeServiceName}
		workspace["runtimeServiceName"], workspace["serviceName"] = resumed.RuntimeServiceName, resumed.RuntimeServiceName
		workspace["receiptId"] = resumed.ReceiptID
		workspace["url"] = resumed.URL
		access := cloneMap(mapField(workspace, "access"))
		credentialsReady := resumed.RuntimeReady && resumed.RuntimeUsername != "" && resumed.CredentialStatus == "configured" && resumed.CredentialSecretRef != ""
		if credentialsReady {
			access["tokenStatus"] = "active"
		} else {
			access["tokenStatus"] = "suspended"
		}
		access["requiresLogin"] = false
		if resumed.RuntimeUsername != "" {
			access["account"], access["username"] = resumed.RuntimeUsername, resumed.RuntimeUsername
		}
		if resumed.CredentialStatus != "" {
			access["credentialStatus"] = resumed.CredentialStatus
		}
		if resumed.CredentialVersion != "" {
			access["credentialVersion"] = resumed.CredentialVersion
		}
		if resumed.CredentialSecretRef != "" {
			access["secretRef"] = resumed.CredentialSecretRef
		}
		workspace["access"] = access
		body := workspaceResponse(cloneMap(workspace))
		result, _ := json.Marshal(map[string]any{"requestHash": requestHash, "response": body})
		operation := map[string]any{"id": operationID, "operationId": operationID, "accountId": accountID, "workspaceId": workspaceID, "resourceId": workspaceID, "resourceKind": "workspace_runtime", "action": "workspace.resume", "provider": stringValue(workspace["provider"]), "providerRequestId": resumed.RuntimeID, "status": "succeeded", "result": string(result), "computeAllocationId": resumed.ComputeID, "storageId": resumed.VolumeID, "attachmentId": resumed.AttachmentID, "runtimeServiceName": resumed.RuntimeServiceName}
		audit := app.auditEvent(r, "workspace.resume", "workspace", workspaceID, accountID, before, body, "succeeded")
		if err := app.tables.CommitWorkspaceResume(r.Context(), workspace, audit, operation); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		writeJSON(w, http.StatusOK, body)
	}))
	mux.HandleFunc("POST /api/workspaces", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		accountID, ok := app.scopedAccountID(w, r, input)
		if !ok {
			return
		}
		attachmentID := stringField(input, "attachmentId", "")
		attachment, ok := app.getAttachment(attachmentID)
		if !ok {
			writeError(w, http.StatusBadRequest, "attached_compute_storage_required")
			return
		}
		if !app.canAccessResource(r, attachment) {
			writeError(w, http.StatusForbidden, "account_scope_forbidden")
			return
		}
		computeID := stringValue(attachment["computeAllocationId"])
		storageID := stringValue(attachment["storageId"])
		workspace, err := service.CreateWorkspace(r.Context(), controlplane.CreateWorkspaceInput{
			AccountID:    accountID,
			OwnerID:      firstNonEmpty(stringField(input, "ownerId", ""), stringField(input, "ownerUserId", "")),
			Name:         firstNonEmpty(stringField(input, "name", ""), stringField(input, "workspaceName", "Workspace")),
			PackageID:    firstNonEmpty(stringField(input, "packageId", ""), stringValue(attachment["packageId"]), "basic"),
			AttachmentID: attachmentID,
			ComputeID:    computeID,
			VolumeID:     storageID,
		}, mutationKey(r, input))
		if err != nil {
			writeUpstreamError(w)
			return
		}
		if err := app.saveWorkspaceProjection(workspace); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		body := workspaceResponse(structToMap(workspace))
		if err := app.appendAuditEvent(r, "workspace.create", "workspace", workspace.ID, workspace.AccountID, nil, body, "succeeded"); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		writeJSON(w, http.StatusCreated, body)
	}))
}
