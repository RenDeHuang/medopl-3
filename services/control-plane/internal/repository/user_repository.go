package repository

import "context"

type UserRepository interface {
	ListUsers(ctx context.Context, includeDeleted bool) ([]map[string]any, error)
	SaveUser(ctx context.Context, user map[string]any) error
	RevokeUserSessions(ctx context.Context, userID string) error
}
