package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"opl-cloud/services/control-plane/internal/controlplane"
)

func NewServer(service *controlplane.Service) http.Handler {
	app := newRuntimeApp()
	mux := http.NewServeMux()
	mux.HandleFunc("/w/", app.proxyWorkspace)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"token": "control-plane-local-token"})
	})
	mux.HandleFunc("POST /api/auth/operator-login", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		expectedToken := strings.TrimSpace(os.Getenv("OPL_OPERATOR_SUMMARY_TOKEN"))
		if expectedToken == "" || stringField(input, "operatorToken", "") != expectedToken {
			writeError(w, http.StatusUnauthorized, "operator_token_invalid")
			return
		}
		session := sessionPayload()
		w.Header().Set("Set-Cookie", "opl_session=control-plane-operator; Path=/; HttpOnly; Secure; SameSite=Lax")
		w.Header().Set("x-opl-csrf-token", session["csrfToken"].(string))
		writeJSON(w, http.StatusOK, session)
	})
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, sessionPayload())
	})
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"id": "usr-local", "role": "owner"})
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.state(r.URL.Query().Get("accountId")))
	})
	mux.HandleFunc("GET /api/management/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.managementState())
	})
	mux.HandleFunc("GET /api/operator/summary", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.operatorSummary())
	})
	mux.HandleFunc("GET /api/runtime/readiness", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.readiness(r.Context()))
	})
	mux.HandleFunc("GET /api/production/readiness", func(w http.ResponseWriter, r *http.Request) {
		readiness := app.readiness(r.Context())
		readiness["checks"] = []any{}
		writeJSON(w, http.StatusOK, readiness)
	})
	mux.HandleFunc("GET /api/overview", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"service": "control-plane", "workspaces": 0})
	})
	mux.HandleFunc("GET /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.state(r.URL.Query().Get("accountId"))["workspaces"])
	})
	mux.HandleFunc("POST /api/workspaces/reset-token", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusOK, map[string]any{"id": stringField(input, "workspaceId", "ws-local"), "tokenStatus": "active"})
	})
	mux.HandleFunc("POST /api/workspaces/delete-token", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusOK, map[string]any{"id": stringField(input, "workspaceId", "ws-local"), "tokenStatus": "deleted"})
	})
	mux.HandleFunc("POST /api/workspaces/runtime-status", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusOK, app.runtimeStatus(r.Context(), stringField(input, "workspaceId", "")))
	})
	mux.HandleFunc("POST /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		idempotencyKey := r.Header.Get("Idempotency-Key")
		_ = idempotencyKey
		workspace, err := app.createWorkspace(r.Context(), decodeJSON(r))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, workspace)
	})
	mux.HandleFunc("GET /api/billing/summary", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"currency": "CNY", "balanceCents": 0})
	})
	mux.HandleFunc("POST /api/billing/topups", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, app.topUp(decodeJSON(r)))
	})
	mux.HandleFunc("POST /api/billing/resource-settlements", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.settleResources(decodeJSON(r)))
	})
	mux.HandleFunc("POST /api/billing/reconciliation", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		report, _ := input["report"].(map[string]any)
		if report == nil {
			report = map[string]any{}
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": stringField(report, "id", "reconciliation-local"), "guard": map[string]any{"status": "ok", "blockNewWorkspaces": false, "reason": "operator_reconciliation"}})
	})
	mux.HandleFunc("GET /api/compute-pools", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, computePools())
	})
	mux.HandleFunc("GET /api/compute-allocations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.state(r.URL.Query().Get("accountId"))["computeAllocations"])
	})
	mux.HandleFunc("POST /api/compute-allocations", func(w http.ResponseWriter, r *http.Request) {
		compute, err := app.createCompute(r.Context(), decodeJSON(r))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, compute)
	})
	mux.HandleFunc("GET /api/compute-allocations/{id}", func(w http.ResponseWriter, r *http.Request) {
		compute, ok := app.getCompute(strings.TrimSpace(r.PathValue("id")))
		if !ok {
			writeError(w, http.StatusNotFound, "compute_allocation_not_found")
			return
		}
		writeJSON(w, http.StatusOK, compute)
	})
	mux.HandleFunc("POST /api/compute-allocations/{id}/destroy", func(w http.ResponseWriter, r *http.Request) {
		compute, err := app.destroyCompute(r.Context(), strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, compute)
	})
	mux.HandleFunc("POST /api/storage-volumes", func(w http.ResponseWriter, r *http.Request) {
		storage, err := app.createStorage(r.Context(), decodeJSON(r))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, storage)
	})
	mux.HandleFunc("POST /api/storage-volumes/destroy", func(w http.ResponseWriter, r *http.Request) {
		storage, err := app.destroyStorage(r.Context(), decodeJSON(r))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, storage)
	})
	mux.HandleFunc("POST /api/storage-attachments", func(w http.ResponseWriter, r *http.Request) {
		attachment, err := app.attachStorage(decodeJSON(r))
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, attachment)
	})
	mux.HandleFunc("POST /api/storage-attachments/detach", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.detachStorage(decodeJSON(r)))
	})
	mux.HandleFunc("GET /api/support/tickets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"tickets": []any{}})
	})
	mux.HandleFunc("POST /api/support/tickets", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusCreated, map[string]any{"id": "ticket-local", "title": stringField(input, "title", "Support"), "status": "open"})
	})
	mux.HandleFunc("GET /api/ledger/task-receipts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"receipts": []any{}})
	})
	mux.HandleFunc("POST /api/organizations", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		name := stringField(input, "name", "Organization")
		writeJSON(w, http.StatusCreated, map[string]any{"id": "org-local", "name": name, "billingAccountId": stringField(input, "billingAccountId", "acct-local"), "status": "active"})
	})
	mux.HandleFunc("POST /api/organizations/members", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusCreated, map[string]any{"id": "membership-local", "organizationId": stringField(input, "organizationId", "org-local"), "userId": stringField(input, "userId", "usr-local"), "role": stringField(input, "role", "member"), "status": "active"})
	})
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusCreated, map[string]any{"id": "usr-local", "email": stringField(input, "email", "owner@example.com"), "accountId": stringField(input, "accountId", "acct-local"), "status": "active"})
	})
	mux.HandleFunc("POST /api/users/disable", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusOK, map[string]any{"id": stringField(input, "userId", "usr-local"), "status": "disabled"})
	})
	mux.HandleFunc("POST /api/users/delete", func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		writeJSON(w, http.StatusOK, map[string]any{"id": stringField(input, "userId", "usr-local"), "status": "deleted"})
	})
	mux.HandleFunc("POST /api/operator/cleanup-workspace-access", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"cleaned": []any{}, "skipped": []any{}})
	})
	mux.HandleFunc("GET /api/admin/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "control-plane", "status": "ok"})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request) map[string]any {
	var input map[string]any
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return map[string]any{}
	}
	return input
}

func stringField(input map[string]any, key string, fallback string) string {
	if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func numberField(input map[string]any, key string, fallback float64) float64 {
	switch value := input[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return fallback
	}
}

func sessionPayload() map[string]any {
	return map[string]any{
		"user":      map[string]any{"id": "usr-local", "email": "owner@example.com", "accountId": "acct-local", "role": "admin", "status": "active"},
		"csrfToken": "csrf-local",
	}
}
