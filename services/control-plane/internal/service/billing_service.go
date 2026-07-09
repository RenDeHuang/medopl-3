package service

import "context"

type BillingService interface {
	Wallet(ctx context.Context, accountID string) (map[string]any, error)
	ManualTopUp(ctx context.Context, input map[string]any, idempotencyKey string) (map[string]any, error)
	SettleResource(ctx context.Context, input map[string]any, idempotencyKey string) (map[string]any, error)
}
