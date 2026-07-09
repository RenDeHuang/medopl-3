package repository

import "context"

type ArchiveRepository interface {
	ArchiveTerminalResources(ctx context.Context, reason string) (map[string]any, error)
	ArchiveState(ctx context.Context) (map[string]any, error)
	ApplyRetention(ctx context.Context, policy any) (map[string]any, error)
}
