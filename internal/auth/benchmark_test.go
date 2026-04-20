package auth

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// BenchmarkOIDCValidator_ValidateToken measures token validation performance
func BenchmarkOIDCValidator_ValidateToken(b *testing.B) {
	if os.Getenv("KEYCLOAK_URL") == "" {
		b.Skip("Skipping: KEYCLOAK_URL not set")
	}

	issuerURL := os.Getenv("KEYCLOAK_URL") + "/realms/localmdm"
	clientID := "local-mdm-api"

	var db *sql.DB
	dsn := os.Getenv("TEST_DSN")
	if dsn != "" {
		var err error
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			b.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	validator, err := NewOIDCValidator(issuerURL, clientID, db, 5, 30*time.Second, 5*time.Minute, logger)
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	token := os.Getenv("TEST_TOKEN")
	if token == "" {
		b.Skip("Skipping: TEST_TOKEN not set")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateToken(token)
	}
}

// BenchmarkCircuitBreaker_Call measures circuit breaker overhead
func BenchmarkCircuitBreaker_Call(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cb := NewCircuitBreaker(5, 30*time.Second, logger)

	successFunc := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Call(successFunc)
	}
}

// BenchmarkTokenCache_Get measures cache read performance
func BenchmarkTokenCache_Get(b *testing.B) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		b.Skip("Database not available")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		b.Skip("Database not available")
	}
	defer db.Close()

	cache, err := NewTokenCache(db, 5*time.Minute)
	if err != nil {
		b.Skip("Token cache not available")
	}

	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}
	ctx := context.Background()
	cache.Set(ctx, "bench-token", user)
	defer cache.Delete(ctx, "bench-token")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "bench-token")
	}
}

// BenchmarkTokenCache_Set measures cache write performance
func BenchmarkTokenCache_Set(b *testing.B) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		b.Skip("Database not available")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		b.Skip("Database not available")
	}
	defer db.Close()

	cache, err := NewTokenCache(db, 5*time.Minute)
	if err != nil {
		b.Skip("Token cache not available")
	}

	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, "bench-token", user)
	}
	cache.Delete(ctx, "bench-token")
}

// BenchmarkWithUser measures context operations
func BenchmarkWithUser(b *testing.B) {
	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithUser(ctx, user)
	}
}

// BenchmarkUserFromContext measures context retrieval
func BenchmarkUserFromContext(b *testing.B) {
	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	ctx := WithUser(context.Background(), user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UserFromContext(ctx)
	}
}
