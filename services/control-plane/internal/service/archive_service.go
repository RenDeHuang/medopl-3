package service

import "context"

type ArchiveService interface {
	ArchiveState(ctx context.Context) (map[string]any, error)
	ArchiveTerminalResources(ctx context.Context, input map[string]any) (map[string]any, error)
	ApplyRetention(ctx context.Context) (map[string]any, error)
}
