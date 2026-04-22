package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=localmdm sslmode=disable")
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTokenCache(t *testing.T) {
	db := getTestDB(t)

	t.Run("connects to database", func(t *testing.T) {
		cache, err := NewTokenCache(db, 5*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		err = cache.Health(ctx)
		assert.NoError(t, err)
	})

	t.Run("stores and retrieves token", func(t *testing.T) {
		cache, err := NewTokenCache(db, 5*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		token := "test-token-" + uuid.New().String()
		user := &AuthUser{
			ID:           "user-123",
			Email:        "test@example.com",
			Roles:        []string{"admin", "user"},
			EnterpriseID: uuid.New(),
		}

		err = cache.Set(ctx, token, user)
		require.NoError(t, err)

		retrieved, err := cache.Get(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, user.Email, retrieved.Email)
		assert.Equal(t, user.Roles, retrieved.Roles)
		assert.Equal(t, user.EnterpriseID, retrieved.EnterpriseID)

		// Cleanup
		_ = cache.Delete(ctx, token)
	})

	t.Run("returns ErrCacheMiss for non-existent token", func(t *testing.T) {
		cache, err := NewTokenCache(db, 5*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = cache.Get(ctx, "non-existent-token-"+uuid.New().String())
		assert.Equal(t, ErrCacheMiss, err)
	})

	t.Run("deletes token", func(t *testing.T) {
		cache, err := NewTokenCache(db, 5*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		token := "test-token-delete-" + uuid.New().String()
		user := &AuthUser{ID: "user-123", Email: "test@example.com"}

		err = cache.Set(ctx, token, user)
		require.NoError(t, err)

		err = cache.Delete(ctx, token)
		require.NoError(t, err)

		_, err = cache.Get(ctx, token)
		assert.Equal(t, ErrCacheMiss, err)
	})

	t.Run("handles concurrent access", func(t *testing.T) {
		cache, err := NewTokenCache(db, 5*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func(i int) {
				defer func() { done <- true }()
				token := "concurrent-" + uuid.New().String()
				user := &AuthUser{ID: "user-" + uuid.New().String(), Email: "test@example.com"}

				_ = cache.Set(ctx, token, user)
				_, _ = cache.Get(ctx, token)
				_ = cache.Delete(ctx, token)
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestTokenCacheErrors(t *testing.T) {
	t.Run("fails with nil database", func(t *testing.T) {
		_, err := NewTokenCache(nil, 5*time.Minute)
		assert.Error(t, err)
	})
}
