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
		"localmdm-api-secret",
	)

	// Get a valid token pair first
	tokenResp, err := kc.Login("admin", "admin123")
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}
	require.NotEmpty(t, tokenResp.RefreshToken)

	// Refresh it
	refreshed, err := kc.RefreshToken(tokenResp.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshed.AccessToken)
	assert.NotEmpty(t, refreshed.RefreshToken)
	assert.Equal(t, "Bearer", refreshed.TokenType)
}

func TestKeycloakRefreshToken_Invalid(t *testing.T) {
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		"localmdm-api-secret",
	)

	_, err := kc.RefreshToken("invalid-refresh-token")
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
	if os.Getenv("DB_HOST") == "" && os.Getenv("KEYCLOAK_URL") == "" {
		t.Skip("skipping: need both DB and Keycloak")
	}

	db := getTestRawDB(t)
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", db, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Skipf("skipping: Keycloak unavailable: %v", err)
	}

	// Validate a real token — exercises the cache Set path
	kc := auth.NewKeycloakClient(keycloakTestURL(), "localmdm-api", "localmdm-api-secret")
	tokenResp, err := kc.Login("admin", "admin123")
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

func getTestRawDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s port=5432 user=postgres password=%s dbname=localmdm sslmode=disable",
		envOrDefault("DB_HOST", "localhost"), envOrDefault("DB_PASSWORD", "postgres"))
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
