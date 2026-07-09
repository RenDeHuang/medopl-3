package service

import "context"

type ResourceService interface {
	CreateCompute(ctx context.Context, input map[string]any, idempotencyKey string) (map[string]any, error)
	DestroyCompute(ctx context.Context, id string, input map[string]any, idempotencyKey string) (map[string]any, error)
	CreateStorage(ctx context.Context, input map[string]any, idempotencyKey string) (map[string]any, error)
	DestroyStorage(ctx context.Context, id string, input map[string]any, idempotencyKey string) (map[string]any, error)
}
