package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenCache(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=1 to run.")
	}

	redisAddr := "localhost:6379"
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		redisAddr = addr
	}

	t.Run("connects to Redis", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 5*time.Minute)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		err = cache.Health(ctx)
		assert.NoError(t, err)
	})

	t.Run("stores and retrieves token", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 5*time.Minute)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		token := "test-token-123"
		user := &AuthUser{
			ID:           "user-123",
			Email:        "test@example.com",
			Roles:        []string{"admin", "user"},
			EnterpriseID: uuid.New(),
		}

		// Store
		err = cache.Set(ctx, token, user)
		require.NoError(t, err)

		// Retrieve
		retrieved, err := cache.Get(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, user.Email, retrieved.Email)
		assert.Equal(t, user.Roles, retrieved.Roles)
		assert.Equal(t, user.EnterpriseID, retrieved.EnterpriseID)
	})

	t.Run("returns ErrCacheMiss for non-existent token", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 5*time.Minute)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		_, err = cache.Get(ctx, "non-existent-token")
		assert.Equal(t, ErrCacheMiss, err)
	})

	t.Run("deletes token", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 5*time.Minute)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		token := "test-token-delete"
		user := &AuthUser{
			ID:    "user-123",
			Email: "test@example.com",
		}

		// Store
		err = cache.Set(ctx, token, user)
		require.NoError(t, err)

		// Delete
		err = cache.Delete(ctx, token)
		require.NoError(t, err)

		// Verify deleted
		_, err = cache.Get(ctx, token)
		assert.Equal(t, ErrCacheMiss, err)
	})

	t.Run("token expires after TTL", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 1*time.Second)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		token := "test-token-ttl"
		user := &AuthUser{
			ID:    "user-123",
			Email: "test@example.com",
		}

		// Store
		err = cache.Set(ctx, token, user)
		require.NoError(t, err)

		// Verify exists
		_, err = cache.Get(ctx, token)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(1500 * time.Millisecond)

		// Verify expired
		_, err = cache.Get(ctx, token)
		assert.Equal(t, ErrCacheMiss, err)
	})

	t.Run("handles concurrent access", func(t *testing.T) {
		cache, err := NewTokenCache(redisAddr, 5*time.Minute)
		require.NoError(t, err)
		defer cache.Close()

		ctx := context.Background()
		done := make(chan bool)

		// Multiple goroutines accessing cache
		for i := 0; i < 10; i++ {
			go func(i int) {
				defer func() { done <- true }()
				token := "test-token-" + string(rune(i))
				user := &AuthUser{
					ID:    "user-" + string(rune(i)),
					Email: "test@example.com",
				}

				// Set
				_ = cache.Set(ctx, token, user)

				// Get
				_, _ = cache.Get(ctx, token)

				// Delete
				_ = cache.Delete(ctx, token)
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should not panic
		assert.NotNil(t, cache)
	})
}

func TestTokenCacheErrors(t *testing.T) {
	t.Run("fails to connect to invalid Redis address", func(t *testing.T) {
		_, err := NewTokenCache("invalid:9999", 5*time.Minute)
		assert.Error(t, err)
	})
}
