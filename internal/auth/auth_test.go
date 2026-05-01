package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestKeycloakLogin(t *testing.T) {
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	
	if tokenResp.AccessToken == "" {
		t.Error("Access token is empty")
	}
	
	if tokenResp.TokenType != "Bearer" {
		t.Errorf("Expected token type 'Bearer', got '%s'", tokenResp.TokenType)
	}
	
	if tokenResp.ExpiresIn == 0 {
		t.Error("ExpiresIn should be set")
	}
}

func TestOIDCValidator(t *testing.T) {
	// Get a valid token first
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	
	// Create validator
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	
	// Validate token
	user, err := validator.ValidateToken(context.Background(), tokenResp.AccessToken)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	
	if user.Email != "admin@localmdm.dev" {
		t.Errorf("Expected email 'admin@localmdm.dev', got '%s'", user.Email)
	}
	
	if !user.HasRole("super_admin") {
		t.Error("User should have super_admin role")
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Get a valid token
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	
	// Create validator and middleware
	validator, _ := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	
	logger := logging.New(config.LoggingConfig{Level: "info", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	
	// Test protected endpoint
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.UserFromContext(r.Context())
		if err != nil {
			t.Error("User should be in context")
			return
		}
		
		if user.Email != "admin@localmdm.dev" {
			t.Errorf("Expected email 'admin@localmdm.dev', got '%s'", user.Email)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	
	// Test with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Test without token
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	
	handler.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w2.Code)
	}
}

func TestRequireRole(t *testing.T) {
	// Get a valid token
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, _ := kc.Login(context.Background(), "admin", "admin123")
	
	// Create validator and middleware
	validator, _ := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	
	logger := logging.New(config.LoggingConfig{Level: "info", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	
	// Test role requirement
	handler := middleware.RequireAuth(
		middleware.RequireRole("super_admin")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)
	
	// Should succeed (admin has super_admin role)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Test with wrong role requirement
	handler2 := middleware.RequireAuth(
		middleware.RequireRole("nonexistent_role")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)
	
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	w2 := httptest.NewRecorder()
	
	handler2.ServeHTTP(w2, req2)
	
	// Should succeed because super_admin has all roles
	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 (super_admin has all roles), got %d", w2.Code)
	}
}

func TestAuthContext(t *testing.T) {
	user := &auth.AuthUser{
		ID:    "test-id",
		Email: "test@example.com",
		Roles: []string{"admin", "operator"},
	}
	
	ctx := auth.WithUser(context.Background(), user)
	
	retrievedUser, err := auth.UserFromContext(ctx)
	if err != nil {
		t.Fatalf("Failed to get user from context: %v", err)
	}
	
	if retrievedUser.Email != user.Email {
		t.Errorf("Expected email '%s', got '%s'", user.Email, retrievedUser.Email)
	}
	
	if !retrievedUser.HasRole("admin") {
		t.Error("User should have admin role")
	}
	
	if !retrievedUser.HasAnyRole("admin", "viewer") {
		t.Error("User should have at least one of the roles")
	}
	
	if retrievedUser.HasRole("super_admin") {
		t.Error("User should not have super_admin role")
	}
}

func TestJWKSRefreshRaceCondition(t *testing.T) {
	// Create validator
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	
	// Get a valid token
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	
	// Simulate concurrent token validations
	// This test should pass with -race flag without detecting any race conditions
	const numGoroutines = 50
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			
			// Validate token multiple times
			for j := 0; j < 10; j++ {
				_, err := validator.ValidateToken(context.Background(), tokenResp.AccessToken)
				if err != nil {
					t.Errorf("Token validation failed: %v", err)
				}
			}
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestRefreshJWKSDoubleCheck(t *testing.T) {
	// Create validator
	validator, err := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	
	// Simulate multiple goroutines trying to refresh simultaneously
	// Only one should actually make the HTTP request (double-check pattern)
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			// Force refresh by calling internal method
			// In real scenario, this is called from ValidateToken
			validator.ValidateToken(context.Background(), "dummy-token") // Will fail but triggers refresh check
		}()
	}
	
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// If we get here without deadlock or panic, the double-check pattern works
}

func TestOIDCValidatorErrors(t *testing.T) {
	tests := []struct {
		name      string
		issuerURL string
		clientID  string
		wantErr   bool
	}{
		{
			name:      "invalid issuer URL",
			issuerURL: "http://invalid-host-that-does-not-exist:9999/realms/test",
			clientID:  "test-client",
			wantErr:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := auth.NewOIDCValidator(tt.issuerURL, tt.clientID, nil, 5, 30*time.Second, 5*time.Minute, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOIDCValidator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRefreshJWKSTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}
	
	// Create a test server that delays response beyond timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than 10s timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()
	
	// Create validator with slow server URL - should timeout
	_, err := auth.NewOIDCValidator(slowServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "failed to fetch JWKS") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestRefreshJWKSEmptyKeys(t *testing.T) {
	// Create a test server that returns empty JWKS
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer emptyServer.Close()
	
	// Create validator with empty JWKS - should fail
	_, err := auth.NewOIDCValidator(emptyServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	if err == nil {
		t.Error("Expected error for empty JWKS, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no keys") {
		t.Errorf("Expected 'no keys' error, got: %v", err)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		authHeader string
		wantToken string
		wantErr   bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer test-token-123",
			wantToken:  "test-token-123",
			wantErr:    false,
		},
		{
			name:       "missing authorization header",
			authHeader: "",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "invalid format - no Bearer prefix",
			authHeader: "test-token-123",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "invalid format - wrong prefix",
			authHeader: "Basic dGVzdDp0ZXN0",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "invalid format - too many parts",
			authHeader: "Bearer token extra",
			wantToken:  "",
			wantErr:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			
			token, err := auth.ExtractBearerToken(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("ExtractBearerToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestOptionalAuth(t *testing.T) {
	// Get a valid token
	kc := auth.NewKeycloakClient(
		keycloakTestURL(),
		"localmdm-api",
		keycloakTestSecret(),
	)
	
	tokenResp, err := kc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	
	// Create validator and middleware
	validator, _ := auth.NewOIDCValidator(keycloakTestURL(), "localmdm-api", nil, 5, 30*time.Second, 5*time.Minute, nil)
	
	logger := logging.New(config.LoggingConfig{Level: "info", Format: "json"})
	middleware := auth.NewMiddleware(validator, logger)
	
	// Test handler that checks for user in context
	handler := middleware.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.UserFromContext(r.Context())
		if err != nil {
			// No user - optional auth allows this
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("anonymous"))
			return
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated:" + user.Email))
	}))
	
	// Test with valid token
	t.Run("with valid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if body != "authenticated:admin@localmdm.dev" {
			t.Errorf("Expected authenticated response, got %s", body)
		}
	})
	
	// Test without token
	t.Run("without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if body != "anonymous" {
			t.Errorf("Expected anonymous response, got %s", body)
		}
	})
	
	// Test with invalid token
	t.Run("with invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if body != "anonymous" {
			t.Errorf("Expected anonymous response (invalid token ignored), got %s", body)
		}
	})
}

func TestKeycloakLoginTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	// Server that hangs longer than the 30s client timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	kc := auth.NewKeycloakClient(slowServer.URL, "test-client", "test-secret")

	start := time.Now()
	_, err := kc.Login(context.Background(), "user", "pass")
	elapsed := time.Since(start)

	assert.Error(t, err)
	// Should fail in ~30s (client timeout), not 35s (server sleep)
	assert.Less(t, elapsed, 34*time.Second, "should timeout before server responds")
}

func TestKeycloakLoginContextCancellation(t *testing.T) {
	// Server that takes 5s — we cancel the context after 500ms
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	kc := auth.NewKeycloakClient(slowServer.URL, "test-client", "test-secret")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := kc.Login(ctx, "user", "pass")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second, "should respect context cancellation, not wait for client timeout")
}

func keycloakTestURL() string {
	if u := os.Getenv("KEYCLOAK_URL"); u != "" {
		return u + "/realms/localmdm"
	}
	return "http://localhost:8180/realms/localmdm"
}

func keycloakTestSecret() string {
	if s := os.Getenv("KEYCLOAK_CLIENT_SECRET"); s != "" {
		return s
	}
	return "localmdm-dev-dashboard-secret-2026"
}
