package repository

import "context"

type ResourceRepository interface {
	ListComputeAllocations(ctx context.Context, accountID string) ([]map[string]any, error)
	ListStorageVolumes(ctx context.Context, accountID string) ([]map[string]any, error)
	ListStorageAttachments(ctx context.Context, accountID string) ([]map[string]any, error)
	SaveComputeAllocation(ctx context.Context, row map[string]any) error
	SaveStorageVolume(ctx context.Context, row map[string]any) error
	SaveStorageAttachment(ctx context.Context, row map[string]any) error
}
