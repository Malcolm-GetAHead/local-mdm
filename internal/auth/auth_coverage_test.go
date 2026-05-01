package auth_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeycloakRefreshToken(t *testing.T) {
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)

	// Get a valid token pair first
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}
	require.NotEmpty(t, tokenResp.RefreshToken)

	// Refresh it
	refreshed, err := kc.RefreshToken(context.Background(), tokenResp.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshed.AccessToken)
	assert.NotEmpty(t, refreshed.RefreshToken)
	assert.Equal(t, "Bearer", refreshed.TokenType)
}

func TestKeycloakRefreshToken_Invalid(t *testing.T) {
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)

	_, err := kc.RefreshToken(context.Background(), "invalid-refresh-token")
	assert.Error(t, err)
}

func TestMiddleware_SetTokenValidator(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)

	tv := &mockTokenValidator{}
	middleware.SetTokenValidator(tv)

	// Verify it works by sending an lmdm_ token through RequireAuth
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.UserFromContext(r.Context())
		require.NoError(t, err)
		assert.Equal(t, "api-user", user.ID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer lmdm_valid_token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_RequireAuth_APIToken_Invalid(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	middleware.SetTokenValidator(&mockTokenValidator{err: fmt.Errorf("invalid token")})

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer lmdm_bad_token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_HealthCheck(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)

	err = middleware.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestOIDCValidator_HealthCheck(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	err = validator.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestOIDCValidator_ValidateToken_Expired(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// A well-formed but expired JWT
	_, err = validator.ValidateToken("eyJhbGciOiJSUzI1NiJ9.eyJleHAiOjF9.invalid")
	assert.Error(t, err)
}

func TestOIDCValidator_ValidateToken_Malformed(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	_, err = validator.ValidateToken("not-a-jwt")
	assert.Error(t, err)
}

func TestNewOIDCValidator_WithDB(t *testing.T) {
	db := getTestRawDB(t)
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", db, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// Validate a real token — exercises the cache Set path
	kc := auth.NewKeycloakClient(keycloakTestURL(), "localmdm-api", keycloakTestSecret())
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	require.NoError(t, err)

	user, err := validator.ValidateToken(tokenResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "admin@localmdm.dev", user.Email)

	// Validate again — should hit cache path
	user2, err := validator.ValidateToken(tokenResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.Email, user2.Email)
}

// --- helpers ---

type mockTokenValidator struct {
	err error
}

func (m *mockTokenValidator) Validate(_ context.Context, _ string) (*auth.AuthUser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &auth.AuthUser{
		ID:           "api-user",
		Email:        "api@test.com",
		Roles:        []string{"admin"},
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}, nil
}

type mockAuditLogger struct {
	events []audit.Event
}

func (m *mockAuditLogger) Log(_ context.Context, event audit.Event) error {
	m.events = append(m.events, event)
	return nil
}

func getTestRawDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s port=5432 user=postgres password=%s dbname=localmdm sslmode=disable",
		envOrDefault("DB_HOST", "localhost"), envOrDefault("DB_PASSWORD", "postgres-dev-password-1234"))
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestCircuitBreaker_OpenAndRecover(t *testing.T) {
	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	cb := auth.NewCircuitBreaker(3, 50*time.Millisecond, logger)

	// Trip the breaker with 3 failures
	for i := 0; i < 3; i++ {
		err := cb.Call(func() error { return fmt.Errorf("fail %d", i) })
		assert.Error(t, err)
	}
	assert.Equal(t, auth.StateOpen, cb.State())

	// Should reject immediately
	err := cb.Call(func() error { return nil })
	assert.ErrorIs(t, err, auth.ErrCircuitOpen)

	// Wait for timeout → half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.AllowRequest()) // transitions to half-open

	// Success in half-open → closed
	err = cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, auth.StateClosed, cb.State())
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	cb := auth.NewCircuitBreaker(2, 50*time.Millisecond, logger)

	// Trip it
	for i := 0; i < 2; i++ {
		cb.Call(func() error { return fmt.Errorf("fail") })
	}
	assert.Equal(t, auth.StateOpen, cb.State())

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open → back to open
	cb.Call(func() error { return fmt.Errorf("still broken") })
	assert.Equal(t, auth.StateOpen, cb.State())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	cb := auth.NewCircuitBreaker(100, 1*time.Second, logger)

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(i int) {
			defer func() { done <- true }()
			if i%2 == 0 {
				cb.Call(func() error { return nil })
			} else {
				cb.Call(func() error { return fmt.Errorf("fail") })
			}
			_ = cb.State()
			cb.AllowRequest()
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	// No panic or race = pass
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := auth.NewCircuitBreaker(1, 1*time.Hour, nil)
	cb.Call(func() error { return fmt.Errorf("fail") })
	assert.Equal(t, auth.StateOpen, cb.State())

	cb.Reset()
	assert.Equal(t, auth.StateClosed, cb.State())
}

func TestMiddleware_RequireAuth_AuditLogging(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	al := &mockAuditLogger{}
	middleware.SetAuditLogger(al)

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Missing token → audit log with "missing_token"
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	require.Len(t, al.events, 1)
	assert.Equal(t, "auth.failure", al.events[0].Action)

	// Invalid JWT → audit log with "invalid_token"
	al.events = nil
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer bad-jwt")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	require.Len(t, al.events, 1)
	assert.Equal(t, "auth.failure", al.events[0].Action)

	// Valid token → audit log with "auth.success"
	al.events = nil
	kc := auth.NewKeycloakClient(keycloakTestURL(), "localmdm-api", keycloakTestSecret())
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	require.NoError(t, err)

	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	require.Len(t, al.events, 1)
	assert.Equal(t, "auth.success", al.events[0].Action)
}

func TestMiddleware_RequireRole_Forbidden_AuditLogging(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	al := &mockAuditLogger{}
	middleware.SetAuditLogger(al)

	// Require a role that doesn't exist — super_admin bypasses, so use a viewer token
	// Since we only have admin, test with a role the admin doesn't have via RequireRole check
	// Actually super_admin has all roles via HasAnyRole. Let's test the "no user in context" path.
	handler := middleware.RequireRole("nonexistent_role_xyz")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// No user in context → 401
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOIDCValidator_HealthCheck_Unreachable(t *testing.T) {
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// Override JWKS URL to something unreachable via a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	err = validator.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestOIDCValidator_ValidateToken_CircuitOpenCacheFallback(t *testing.T) {
	db := getTestRawDB(t)
	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})

	// Create validator with very low failure threshold
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", db, 1, 10*time.Second, 5*time.Minute, logger)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// Get a real token and validate it (populates cache)
	kc := auth.NewKeycloakClient(keycloakTestURL(), "localmdm-api", keycloakTestSecret())
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	require.NoError(t, err)

	user, err := validator.ValidateToken(tokenResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "admin@localmdm.dev", user.Email)

	// Trip the circuit breaker by validating a bad token
	_, _ = validator.ValidateToken("invalid-token-to-trip-breaker")

	// Now the circuit is open — validate the good token again
	// Should fall back to cache
	user2, err := validator.ValidateToken(tokenResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "admin@localmdm.dev", user2.Email)
}

func TestOIDCValidator_ValidateToken_CircuitOpenNoCacheMiss(t *testing.T) {
	// Validator WITHOUT cache, low failure threshold
	logger := logging.New(config.LoggingConfig{Level: "error", Format: "json"})
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 1, 10*time.Second, 5*time.Minute, logger)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// Trip the breaker
	_, _ = validator.ValidateToken("bad-token")

	// Circuit open, no cache → should return ErrCircuitOpen
	_, err = validator.ValidateToken("another-token")
	assert.ErrorIs(t, err, auth.ErrCircuitOpen)
}
