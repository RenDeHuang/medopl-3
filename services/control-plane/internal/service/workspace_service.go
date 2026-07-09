package service

import "context"

type WorkspaceService interface {
	ListWorkspaces(ctx context.Context, accountID string) ([]map[string]any, error)
	CreateWorkspace(ctx context.Context, input map[string]any, idempotencyKey string) (map[string]any, error)
	SetWorkspaceAccess(ctx context.Context, workspaceID string, tokenStatus string) (map[string]any, error)
}
