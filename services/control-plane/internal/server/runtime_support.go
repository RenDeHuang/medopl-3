package server

import (
	"errors"
	"strings"
	"time"
)

func (app *controlPlaneApp) supportTickets(scopeAll bool, accountID string) []any {
	app.mu.Lock()
	defer app.mu.Unlock()
	if scopeAll || accountID == "" {
		return values(app.support)
	}
	return filteredValues(app.support, func(item map[string]any) bool {
		return stringValue(item["accountId"]) == accountID
	})
}

func (app *controlPlaneApp) createSupportMapping(input map[string]any) (map[string]any, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	externalTicketID := stringField(input, "externalTicketId", "")
	if externalTicketID == "" {
		return nil, errors.New("missing_external_ticket_id")
	}
	accountID := stringField(input, "accountId", "")
	if accountID == "" {
		return nil, errors.New("missing_account_id")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := "support-" + stableID(accountID, externalTicketID)[:12]
	message := strings.TrimSpace(stringField(input, "description", ""))
	row := map[string]any{
		"id":               id,
		"externalSystem":   stringField(input, "externalSystem", "external-helpdesk"),
		"externalTicketId": externalTicketID,
		"externalUrl":      stringField(input, "externalUrl", ""),
		"accountId":        accountID,
		"userId":           stringField(input, "userId", ""),
		"workspaceId":      stringField(input, "workspaceId", ""),
		"resourceIds":      stringSliceField(input, "resourceIds"),
		"operationId":      stringField(input, "operationId", ""),
		"title":            stringField(input, "title", externalTicketID),
		"category":         stringField(input, "category", "Workspace"),
		"priority":         stringField(input, "priority", "normal"),
		"status":           stringField(input, "status", "external_open"),
		"createdAt":        now,
		"updatedAt":        now,
		"messages":         []any{},
	}
	if message != "" {
		row["messages"] = []any{map[string]any{"author": "requester", "text": message, "createdAt": now}}
	}
	app.support[id] = row
	return cloneMap(row), app.persistLocked()
}
