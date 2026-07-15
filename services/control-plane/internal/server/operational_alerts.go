package server

import (
	"log"
	"slices"
	"sort"
	"strings"
)

var operationalAlertCodes = [...]string{"manual_review", "past_due", "ledger_receipt_pending", "cleanup_failed"}

func monthlyOperationalAlertCodes(row map[string]any) []string {
	codes := make([]string, 0, len(operationalAlertCodes))
	status, lastError := stringValue(row["billingStatus"]), stringValue(row["lastBillingError"])
	if status == "manual_review" {
		codes = append(codes, "manual_review")
	}
	if status == "past_due" {
		codes = append(codes, "past_due")
	}
	if lastError == "ledger_receipt_pending" {
		codes = append(codes, "ledger_receipt_pending")
	}
	if strings.HasSuffix(lastError, "_cleanup_failed") || lastError == "fabric_expiry_destroy_failed" {
		codes = append(codes, "cleanup_failed")
	}
	return codes
}

func operationalNotificationSummary(computes, storages controlPlaneRecordSet) map[string]any {
	recent := make([]any, 0)
	errorCount, warningCount := 0, 0
	appendRows := func(resourceType string, rows controlPlaneRecordSet) {
		for _, row := range rows {
			for _, code := range monthlyOperationalAlertCodes(row) {
				severity := "error"
				if code == "past_due" || code == "ledger_receipt_pending" {
					severity = "warning"
					warningCount++
				} else {
					errorCount++
				}
				recent = append(recent, map[string]any{
					"id":           "alert-" + stableID(code, resourceType, stringValue(row["id"]))[:12],
					"type":         code,
					"code":         code,
					"severity":     severity,
					"resourceType": resourceType,
					"resourceId":   stringValue(row["id"]),
					"accountId":    firstNonEmpty(stringValue(row["accountId"]), stringValue(row["ownerAccountId"])),
					"workspaceId":  stringValue(row["workspaceId"]),
				})
			}
		}
	}
	appendRows("compute", computes)
	appendRows("storage", storages)
	sort.Slice(recent, func(i, j int) bool {
		left, right := recent[i].(map[string]any), recent[j].(map[string]any)
		return stringValue(left["id"]) < stringValue(right["id"])
	})
	return map[string]any{"total": len(recent), "error": errorCount, "warning": warningCount, "recent": recent}
}

func (app *controlPlaneServer) observeMonthlyOperationalAlerts(resourceType string, row map[string]any) {
	id := stringValue(row["id"])
	if id == "" {
		return
	}
	activeCodes := monthlyOperationalAlertCodes(row)
	resourceRef := stableID(resourceType, id)[:12]
	for _, code := range operationalAlertCodes {
		key := resourceType + ":" + id + ":" + code
		if slices.Contains(activeCodes, code) {
			if _, loaded := app.operationalAlertStates.LoadOrStore(key, struct{}{}); !loaded {
				log.Printf("event=opl_operational_state code=%s state=active resource_type=%s resource_ref=%s", code, resourceType, resourceRef)
			}
			continue
		}
		if _, loaded := app.operationalAlertStates.LoadAndDelete(key); loaded {
			log.Printf("event=opl_operational_state code=%s state=recovered resource_type=%s resource_ref=%s", code, resourceType, resourceRef)
		}
	}
}
