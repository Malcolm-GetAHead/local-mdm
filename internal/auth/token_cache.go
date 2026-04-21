package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrCacheMiss   = errors.New("token not found in cache")
)

// TokenCache caches validated tokens in PostgreSQL
type TokenCache struct {
	db  *sql.DB
	ttl time.Duration
}

// NewTokenCache creates a new PostgreSQL-backed token cache
func NewTokenCache(db *sql.DB, ttl time.Duration) (*TokenCache, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &TokenCache{db: db, ttl: ttl}, nil
}

// hashToken returns a SHA-256 hex digest of the token
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Get retrieves a cached token
func (tc *TokenCache) Get(ctx context.Context, token string) (*AuthUser, error) {
	hash := hashToken(token)

	var data []byte
	err := tc.db.QueryRowContext(ctx,
		`SELECT user_data FROM token_cache WHERE token_hash = $1 AND expires_at > NOW()`,
		hash,
	).Scan(&data)
	if err == sql.ErrNoRows {
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
	hash := hashToken(token)

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	_, err = tc.db.ExecContext(ctx,
		`INSERT INTO token_cache (token_hash, user_data, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (token_hash) DO UPDATE SET user_data = $2, expires_at = $3`,
		hash, data, time.Now().Add(tc.ttl),
	)
	if err != nil {
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	return nil
}

// Delete removes a token from the cache
func (tc *TokenCache) Delete(ctx context.Context, token string) error {
	hash := hashToken(token)
	_, err := tc.db.ExecContext(ctx, `DELETE FROM token_cache WHERE token_hash = $1`, hash)
	return err
}

// Close is a no-op for PostgreSQL (connection managed externally)
func (tc *TokenCache) Close() error {
	return nil
}

// Health checks if the database is healthy
func (tc *TokenCache) Health(ctx context.Context) error {
	return tc.db.PingContext(ctx)
}

// CleanupExpired removes expired entries. Call periodically.
func (tc *TokenCache) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := tc.db.ExecContext(ctx, `DELETE FROM token_cache WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
