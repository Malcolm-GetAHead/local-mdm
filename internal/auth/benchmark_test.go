package auth

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// BenchmarkOIDCValidator_ValidateToken measures token validation performance
func BenchmarkOIDCValidator_ValidateToken(b *testing.B) {
	// Skip if no Keycloak available
	if os.Getenv("KEYCLOAK_URL") == "" {
		b.Skip("Skipping: KEYCLOAK_URL not set")
	}

	issuerURL := os.Getenv("KEYCLOAK_URL") + "/realms/localmdm"
	clientID := "local-mdm-api"
	redisAddr := "localhost:6379"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	validator, err := NewOIDCValidator(issuerURL, clientID, redisAddr, 5, 30*time.Second, 5*time.Minute, logger)
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	// Get a valid token for testing
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
	cache, err := NewTokenCache("localhost:6379", 5*time.Minute)
	if err != nil {
		b.Skip("Redis not available")
	}

	// Pre-populate cache
	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}
	ctx := context.Background()
	cache.Set(ctx, "test-token", user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "test-token")
	}
}

// BenchmarkTokenCache_Set measures cache write performance
func BenchmarkTokenCache_Set(b *testing.B) {
	cache, err := NewTokenCache("localhost:6379", 5*time.Minute)
	if err != nil {
		b.Skip("Redis not available")
	}

	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, "test-token", user)
	}
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
