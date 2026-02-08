package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type contextKey string

const (
	userContextKey contextKey = "auth_user"
	requestIDKey   contextKey = "request_id"
)

// AuthUser represents an authenticated user with their roles and enterprise context.
// It is stored in the request context after successful authentication.
type AuthUser struct {
	ID           string
	Email        string
	Roles        []string
	EnterpriseID uuid.UUID
}

func (u *AuthUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
		if r == "super_admin" {
			return true // super_admin has all roles
		}
	}
	return false
}

func (u *AuthUser) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// WithUser returns a new context with the authenticated user attached.
// The user can be retrieved later using UserFromContext.
func WithUser(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context.
// Returns an error if no user is found in the context.
func UserFromContext(ctx context.Context) (*AuthUser, error) {
	user, ok := ctx.Value(userContextKey).(*AuthUser)
	if !ok || user == nil {
		return nil, fmt.Errorf("no user in context")
	}
	return user, nil
}

