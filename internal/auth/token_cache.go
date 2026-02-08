package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrCacheMiss   = errors.New("token not found in cache")
)

// TokenCache caches validated tokens in Redis
type TokenCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewTokenCache creates a new token cache
func NewTokenCache(redisAddr string, ttl time.Duration) (*TokenCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &TokenCache{
		client: client,
		ttl:    ttl,
	}, nil
}

// Get retrieves a cached token
func (tc *TokenCache) Get(ctx context.Context, token string) (*AuthUser, error) {
	key := fmt.Sprintf("token:%s", token)
	
	data, err := tc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var user AuthUser
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user: %w", err)
	}

	return &user, nil
}

// Set stores a token in the cache
func (tc *TokenCache) Set(ctx context.Context, token string, user *AuthUser) error {
	key := fmt.Sprintf("token:%s", token)
	
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	if err := tc.client.Set(ctx, key, data, tc.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	return nil
}

// Delete removes a token from the cache
func (tc *TokenCache) Delete(ctx context.Context, token string) error {
	key := fmt.Sprintf("token:%s", token)
	return tc.client.Del(ctx, key).Err()
}

// Close closes the Redis connection
func (tc *TokenCache) Close() error {
	return tc.client.Close()
}

// Health checks if Redis is healthy
func (tc *TokenCache) Health(ctx context.Context) error {
	return tc.client.Ping(ctx).Err()
}
