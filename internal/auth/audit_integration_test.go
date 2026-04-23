package auth

import (
	"os"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDBForAudit(t *testing.T) *db.DB {
	t.Helper()

	cfg := config.DatabaseConfig{
		Host:            func() string { if h := os.Getenv("DB_HOST"); h != "" { return h }; return "localhost" }(),
		Port:            5432,
		User:            "postgres",
		Password:        func() string { if p := os.Getenv("DB_PASSWORD"); p != "" { return p }; return "postgres" }(),
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := db.New(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	return database
}

func TestMiddleware_AuditLogging_AuthFailure(t *testing.T) {
	database := setupTestDBForAudit(t)
	defer database.Close()

	// Create middleware with audit logger
	validator := &OIDCValidator{} // Mock validator
	logger := slog.Default()
	middleware := NewMiddleware(validator, logger)
	auditLogger := audit.NewLogger(database.Writer)
	middleware.SetAuditLogger(auditLogger)

	// Create test handler
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make request without auth token
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Verify audit log was created
	time.Sleep(100 * time.Millisecond) // Give DB time to write

	var count int
	err := database.Writer.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE action = $1 AND created_at > NOW() - INTERVAL '5 seconds'",
		"auth.failure",
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "Expected at least one auth.failure audit log")

	// Cleanup
	_, _ = database.Writer.ExecContext(context.Background(),
		"DELETE FROM audit_logs WHERE action = $1 AND created_at > NOW() - INTERVAL '5 seconds'",
		"auth.failure",
	)
}

func TestMiddleware_AuditLogging_AccessDenied(t *testing.T) {
	database := setupTestDBForAudit(t)
	defer database.Close()

	// Create middleware with audit logger
	validator := &OIDCValidator{} // Mock validator
	logger := slog.Default()
	middleware := NewMiddleware(validator, logger)
	auditLogger := audit.NewLogger(database.Writer)
	middleware.SetAuditLogger(auditLogger)

	// Create test handler with role requirement
	handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create request with user context (but wrong role)
	req := httptest.NewRequest("GET", "/test", nil)
	user := &AuthUser{
		ID:    "test-user",
		Email: "test@example.com",
		Roles: []string{"user"}, // Not admin
	}
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Verify audit log was created
	time.Sleep(100 * time.Millisecond) // Give DB time to write

	var count int
	err := database.Writer.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE action = $1 AND created_at > NOW() - INTERVAL '5 seconds'",
		"auth.access_denied",
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "Expected at least one auth.access_denied audit log")

	// Cleanup
	_, _ = database.Writer.ExecContext(context.Background(),
		"DELETE FROM audit_logs WHERE action = $1 AND created_at > NOW() - INTERVAL '5 seconds'",
		"auth.access_denied",
	)
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remoteAddr string
		expected string
	}{
		{
			name: "X-Forwarded-For single IP",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.1",
		},
		{
			name: "X-Forwarded-For multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.1",
		},
		{
			name: "X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.2",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "203.0.113.3:12345",
			expected:   "203.0.113.3",
		},
		{
			name: "X-Forwarded-For takes precedence",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.4",
				"X-Real-IP":       "203.0.113.5",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remoteAddr

			ip := getIP(req)
			require.NotNil(t, ip)
			assert.Equal(t, tt.expected, ip.String())
		})
	}
}
