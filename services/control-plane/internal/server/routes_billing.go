package server

import (
	"net/http"
	"strconv"
	"strings"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/controlplane"
)

func registerBillingRoutes(mux *http.ServeMux, app *controlPlaneServer, service *controlplane.Service) {
	mux.HandleFunc("GET /api/billing/summary", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := app.scopedAccountID(w, r, nil)
		if !ok {
			return
		}
		balance, ok := app.liveBalance(w, r, service, accountID, false)
		if ok {
			writeJSON(w, http.StatusOK, balance)
		}
	}))
	mux.HandleFunc("GET /api/billing/receipts", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := app.scopedAccountID(w, r, nil)
		if !ok {
			return
		}
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(w, http.StatusBadRequest, "invalid_receipt_query")
				return
			}
			limit = parsed
		}
		page, err := service.BillingReceipts(r.Context(), clients.ReceiptListQuery{AccountID: accountID, Cursor: r.URL.Query().Get("cursor"), Limit: limit})
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		items := make([]any, 0, len(page.Receipts))
		for _, receipt := range page.Receipts {
			if receipt.AccountID != accountID || !strings.HasPrefix(receipt.Type, "billing.") {
				continue
			}
			items = append(items, customerBillingReceipt(receipt))
		}
		writeJSON(w, http.StatusOK, map[string]any{"receipts": items, "nextCursor": page.NextCursor, "hasMore": page.HasMore})
	}))
	mux.HandleFunc("GET /api/billing/receipts/{id}", app.protected(false, func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := app.scopedAccountID(w, r, nil)
		if !ok {
			return
		}
		receipt, err := service.BillingReceipt(r.Context(), strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		if receipt.AccountID != accountID || !strings.HasPrefix(receipt.Type, "billing.") {
			writeError(w, http.StatusNotFound, "billing_receipt_not_found")
			return
		}
		writeJSON(w, http.StatusOK, customerBillingReceipt(receipt))
	}))
	mux.HandleFunc("POST /api/billing/reconciliation", app.protected(true, func(w http.ResponseWriter, r *http.Request) {
		input := decodeJSON(r)
		if !confirmed(input, "confirm") {
			writeError(w, http.StatusBadRequest, "confirmation_required")
			return
		}
		report, _ := input["report"].(map[string]any)
		if report == nil {
			report = map[string]any{}
		}
		idempotencyKey := firstNonEmpty(r.Header.Get("Idempotency-Key"), stringField(input, "idempotencyKey", ""), stringField(report, "id", ""))
		if idempotencyKey == "" {
			writeError(w, http.StatusBadRequest, "missing Idempotency-Key")
			return
		}
		result, err := service.RecordReconciliation(r.Context(), controlplane.ReconciliationInput{Report: report}, idempotencyKey)
		if err != nil {
			writeUpstreamError(w, err)
			return
		}
		if err := app.rememberReconciliation(result); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		if err := app.appendAuditEvent(r, "billing.reconciliation", "billing_reconciliation", stringField(report, "id", ""), "", nil, result, "succeeded"); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed")
			return
		}
		writeJSON(w, http.StatusCreated, reconciliationResponse(result))
	}))
}

func customerBillingReceipt(receipt clients.Receipt) map[string]any {
	cost := receipt.Cost
	return map[string]any{
		"receiptId": receipt.ReceiptID, "type": receipt.Type, "status": receipt.Status,
		"workspaceId": receipt.WorkspaceID, "createdAt": receipt.CreatedAt,
		"resourceType": cost["resourceType"], "resourceId": cost["resourceId"],
		"chargeUsdMicros": cost["chargeUsdMicros"], "periodStart": cost["periodStart"], "paidThrough": cost["paidThrough"],
	}
}
