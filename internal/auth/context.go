package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type contextKey string

const userContextKey contextKey = "auth_user"

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

func WithUser(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*AuthUser, error) {
	user, ok := ctx.Value(userContextKey).(*AuthUser)
	if !ok || user == nil {
		return nil, fmt.Errorf("no user in context")
	}
	return user, nil
}

func MustUserFromContext(ctx context.Context) *AuthUser {
	user, err := UserFromContext(ctx)
	if err != nil {
		panic(err)
	}
	return user
}
