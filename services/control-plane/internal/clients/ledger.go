package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type LedgerClient interface {
	CreateHold(ctx context.Context, input HoldInput, idempotencyKey string) (HoldResult, error)
	RecordEvidence(ctx context.Context, input EvidenceInput, idempotencyKey string) (EvidenceReceipt, error)
}

type HoldInput struct {
	AccountID   string `json:"accountId"`
	WorkspaceID string `json:"workspaceId"`
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
}

type HoldResult struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AmountCents int64  `json:"amountCents"`
}

type EvidenceInput struct {
	WorkspaceID       string `json:"workspaceId"`
	ProviderRequestID string `json:"providerRequestId"`
	RedactedURL       string `json:"redactedUrl"`
	TokenVersion      string `json:"tokenVersion"`
}

type EvidenceReceipt struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
}

type ledgerHTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewLedgerHTTPClient(baseURL string, client *http.Client) LedgerClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &ledgerHTTPClient{baseURL: baseURL, client: client}
}

func (c *ledgerHTTPClient) CreateHold(ctx context.Context, input HoldInput, idempotencyKey string) (HoldResult, error) {
	var result HoldResult
	err := c.post(ctx, "/ledger/holds", input, idempotencyKey, &result)
	return result, err
}

func (c *ledgerHTTPClient) RecordEvidence(ctx context.Context, input EvidenceInput, idempotencyKey string) (EvidenceReceipt, error) {
	var result EvidenceReceipt
	err := c.post(ctx, "/ledger/evidence", input, idempotencyKey, &result)
	return result, err
}

func (c *ledgerHTTPClient) post(ctx context.Context, path string, input any, idempotencyKey string, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(output)
}
