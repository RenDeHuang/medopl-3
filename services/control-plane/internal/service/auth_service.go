package service

import "context"

type AuthService interface {
	Login(ctx context.Context, input map[string]any) (map[string]any, string, error)
	Logout(ctx context.Context, sessionID string) error
	CurrentUser(ctx context.Context, sessionID string) (map[string]any, error)
}
