package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerStartupFailsWithInvalidKeycloak verifies that server creation fails
// when Keycloak URL is invalid or unreachable, preventing authentication bypass
func TestServerStartupFailsWithInvalidKeycloak(t *testing.T) {
	tests := []struct {
		name        string
		keycloakURL string
		wantErr     bool
		errContains string
	}{
		{
			name:        "invalid URL",
			keycloakURL: "http://invalid-keycloak:9999",
			wantErr:     true,
			errContains: "CRITICAL: Cannot start server without authentication",
		},
		{
			name:        "malformed URL",
			keycloakURL: "not-a-url",
			wantErr:     true,
			errContains: "CRITICAL: Cannot start server without authentication",
		},
		{
			name:        "empty URL",
			keycloakURL: "",
			wantErr:     true,
			errContains: "CRITICAL: Cannot start server without authentication",
		},
		{
			name:        "unreachable host",
			keycloakURL: "http://192.0.2.1:8080", // TEST-NET-1, guaranteed unreachable
			wantErr:     true,
			errContains: "CRITICAL: Cannot start server without authentication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal mock database that satisfies the interface
			// We don't need actual DB for this test - just testing auth initialization
			mockDB := &db.DB{}

			// Create config with invalid Keycloak URL
			cfg := &config.Config{
				Server: config.ServerConfig{
					Host: func() string { if h := os.Getenv("DB_HOST"); h != "" { return h }; return "localhost" }(),
					Port: 8080,
				},
				Keycloak: config.KeycloakConfig{
					URL:      tt.keycloakURL,
					Realm:    "test",
					ClientID: "test-client",
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelError, // Suppress logs during test
			}))

			// Attempt to create server
			server, err := New(cfg, mockDB, logger)

			if tt.wantErr {
				require.Error(t, err, "Expected server creation to fail")
				assert.Contains(t, err.Error(), tt.errContains, "Error message should indicate critical auth failure")
				assert.Nil(t, server, "Server should be nil on error")
			} else {
				require.NoError(t, err, "Expected server creation to succeed")
				assert.NotNil(t, server, "Server should not be nil")
			}
		})
	}
}

// TestProtectedRoutesRequireAuth verifies that all protected routes
// return 401 when accessed without authentication
func TestProtectedRoutesRequireAuth(t *testing.T) {
	// Setup test server with valid Keycloak (mocked)
	server := setupTestServer(t)

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/enterprises"},
		{"POST", "/api/v1/enterprises"},
		{"GET", "/api/v1/enterprises/00000000-0000-0000-0000-000000000001"},
		{"GET", "/api/v1/devices"},
		{"GET", "/api/v1/devices/00000000-0000-0000-0000-000000000001"},
		{"POST", "/api/v1/devices/00000000-0000-0000-0000-000000000001/lock"},
		{"POST", "/api/v1/devices/00000000-0000-0000-0000-000000000001/wipe"},
		{"GET", "/api/v1/policies"},
		{"POST", "/api/v1/policies"},
		{"GET", "/api/v1/policies/00000000-0000-0000-0000-000000000001"},
		{"GET", "/api/v1/certificates"},
		{"GET", "/api/v1/audit-logs"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			// No Authorization header
			rr := httptest.NewRecorder()

			server.router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code,
				"Protected route should return 401 without auth")
		})
	}
}

// TestPublicRoutesAccessibleWithoutAuth verifies that public routes
// are accessible without authentication
func TestPublicRoutesAccessibleWithoutAuth(t *testing.T) {
	server := setupTestServer(t)

	publicRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/version"},
	}

	for _, route := range publicRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rr := httptest.NewRecorder()

			server.router.ServeHTTP(rr, req)

			assert.NotEqual(t, http.StatusUnauthorized, rr.Code,
				"Public route should be accessible without auth")
		})
	}
}

// TestAuthMiddlewareNotNil verifies that authMiddleware is always set
// after successful server creation
func TestAuthMiddlewareNotNil(t *testing.T) {
	server := setupTestServer(t)
	assert.NotNil(t, server.authMiddleware,
		"authMiddleware must not be nil after server creation")
}

// TestServerCreationWithValidKeycloak verifies that server creation succeeds
// with a valid Keycloak configuration
func TestServerCreationWithValidKeycloak(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Use a mock Keycloak server
	mockKeycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/test/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"issuer": "` + r.Host + `/realms/test",
				"jwks_uri": "` + r.Host + `/realms/test/protocol/openid-connect/certs"
			}`))
		} else if r.URL.Path == "/realms/test/protocol/openid-connect/certs" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"keys": [{"kty": "RSA", "use": "sig", "kid": "test-key", "alg": "RS256", "n": "test", "e": "AQAB"}]}`))
		}
	}))
	defer mockKeycloak.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: func() string { if h := os.Getenv("DB_HOST"); h != "" { return h }; return "localhost" }(),
			Port: 8080,
		},
		Keycloak: config.KeycloakConfig{
			URL:      mockKeycloak.URL,
			Realm:    "test",
			ClientID: "test-client",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	server, err := New(cfg, testDB, logger)
	require.NoError(t, err, "Server creation should succeed with valid Keycloak")
	assert.NotNil(t, server, "Server should not be nil")
	assert.NotNil(t, server.authMiddleware, "Auth middleware should be initialized")
}

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()

	cfg := config.DatabaseConfig{
		Host:            func() string { if h := os.Getenv("DB_HOST"); h != "" { return h }; return "localhost" }(),
		Port:            5432,
		User:            "postgres",
		Password:        func() string { if p := os.Getenv("DB_PASSWORD"); p != "" { return p }; return "postgres" }(),
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300000000000, // 5 minutes in nanoseconds
	}

	database, err := db.New(cfg)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}

	return database
}

// setupTestServer creates a test server with mocked Keycloak
func setupTestServer(t *testing.T) *Server {
	t.Helper()

	testDB := setupTestDB(t)
	t.Cleanup(func() { testDB.Close() })

	// Mock Keycloak server
	mockKeycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/test/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"issuer": "http://` + r.Host + `/realms/test",
				"jwks_uri": "http://` + r.Host + `/realms/test/protocol/openid-connect/certs"
			}`))
		} else if r.URL.Path == "/realms/test/protocol/openid-connect/certs" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"keys": [{"kty": "RSA", "use": "sig", "kid": "test-key", "alg": "RS256", "n": "test", "e": "AQAB"}]}`))
		}
	}))
	t.Cleanup(mockKeycloak.Close)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: func() string { if h := os.Getenv("DB_HOST"); h != "" { return h }; return "localhost" }(),
			Port: 8080,
		},
		Keycloak: config.KeycloakConfig{
			URL:      mockKeycloak.URL,
			Realm:    "test",
			ClientID: "test-client",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	server, err := New(cfg, testDB, logger)
	require.NoError(t, err, "Failed to create test server")

	return server
}

// TestAuthEndpointRateLimiting verifies that authentication endpoints have strict rate limiting
func TestAuthEndpointRateLimiting(t *testing.T) {
	t.Run("IP-based rate limiting on login", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Shutdown(nil)

		// Use different usernames to avoid account-based limit
		// IP limit is 10 per minute

		for i := 0; i < 10; i++ {
			loginReq := map[string]string{
				"username": fmt.Sprintf("user%d@example.com", i),
				"password": "password123",
			}
			body, _ := json.Marshal(loginReq)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = "192.168.1.100:12345"
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not be rate limited yet
			if w.Code == http.StatusTooManyRequests {
				t.Fatalf("request %d should not be rate limited (IP limit is 10), got status %d", i+1, w.Code)
			}
		}

		// 11th request should be rate limited (IP-based)
		loginReq := map[string]string{
			"username": "user11@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(loginReq)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code, "11th request should be rate limited")
		assert.NotEmpty(t, w.Header().Get("Retry-After"), "Retry-After header should be set")

		var response struct {
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.Error)
		assert.Equal(t, "rate_limit_exceeded", response.Error.Code)
		assert.Contains(t, response.Error.Message, "IP", "error message should mention IP-based limit")
	})

	t.Run("Account-based rate limiting on login", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Shutdown(nil)

		loginReq := map[string]string{
			"username": "limited@example.com",
			"password": "password123",
		}

		// Make requests up to account limit (5 per 5 minutes) from different IPs
		for i := 0; i < 5; i++ {
			body, _ := json.Marshal(loginReq)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = fmt.Sprintf("192.168.2.%d:12345", i+1)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not be rate limited yet
			if w.Code == http.StatusTooManyRequests {
				t.Fatalf("request %d should not be rate limited (account limit is 5), got status %d", i+1, w.Code)
			}
		}

		// 6th request should be rate limited (account-based)
		body, _ := json.Marshal(loginReq)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "192.168.2.100:12345" // Different IP
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code, "6th request should be rate limited (account)")
		
		var response struct {
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.Error)
		assert.Equal(t, "rate_limit_exceeded", response.Error.Code)
		assert.Contains(t, response.Error.Message, "account", "error message should mention account-based limit")
	})

	t.Run("Different IPs have independent limits", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Shutdown(nil)

		loginReq := map[string]string{
			"username": "test@example.com",
			"password": "password123",
		}

		// Each IP should be able to make requests independently
		for i := 0; i < 5; i++ {
			body, _ := json.Marshal(loginReq)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = fmt.Sprintf("10.0.0.%d:12345", i+1)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not be rate limited (different IPs)
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("request from IP %d should not be rate limited, got status %d", i+1, w.Code)
			}
		}
	})

	t.Run("Refresh endpoint has rate limiting", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Shutdown(nil)

		refreshReq := map[string]string{
			"refresh_token": "fake-token",
		}

		// Make requests up to IP limit
		for i := 0; i < 10; i++ {
			body, _ := json.Marshal(refreshReq)
			req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
			req.RemoteAddr = "192.168.3.100:12345"
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should not be rate limited yet
			if w.Code == http.StatusTooManyRequests {
				t.Fatalf("refresh request %d should not be rate limited, got status %d", i+1, w.Code)
			}
		}

		// 11th request should be rate limited
		body, _ := json.Marshal(refreshReq)
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(body))
		req.RemoteAddr = "192.168.3.100:12345"
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code, "11th refresh request should be rate limited")
	})

	t.Run("Rate limit stricter than global limit", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Shutdown(nil)

		// Auth endpoints should have 10 req/min IP limit and 5 req/5min account limit
		// The account limit (5) will be hit first when using same username
		// This test verifies auth-specific limits are enforced

		loginReq := map[string]string{
			"username": "test@example.com",
			"password": "password123",
		}

		successCount := 0
		for i := 0; i < 20; i++ {
			body, _ := json.Marshal(loginReq)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = "192.168.4.100:12345"
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusTooManyRequests {
				successCount++
			}
		}

		// Should allow exactly 5 requests (account limit), not 10 (IP limit) or 20 (would be global limit)
		assert.Equal(t, 5, successCount, "should enforce auth-specific account limit of 5")
	})
}
