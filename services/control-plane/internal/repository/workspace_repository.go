package repository

import "context"

type WorkspaceRepository interface {
	ListWorkspaces(ctx context.Context, accountID string) ([]map[string]any, error)
	SaveWorkspace(ctx context.Context, row map[string]any) error
	SetWorkspaceAccess(ctx context.Context, workspaceID string, tokenStatus string) (map[string]any, error)
}
