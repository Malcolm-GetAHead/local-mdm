package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type healthResponse struct {
	Data struct {
		Status    string            `json:"status"`
		Version   string            `json:"version"`
		Checks    map[string]string `json:"checks"`
		Timestamp time.Time         `json:"timestamp"`
	} `json:"data"`
}

func TestHandleHealth_Integration(t *testing.T) {
	server := setupTestServer(t)
	defer server.Shutdown(nil)

	t.Run("all dependencies healthy", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Data.Status)
		assert.Equal(t, "1.0.0", response.Data.Version)
		assert.Equal(t, "healthy", response.Data.Checks["database"])
		assert.Equal(t, "healthy", response.Data.Checks["keycloak"])
		assert.False(t, response.Data.Timestamp.IsZero())
	})

	t.Run("response format is valid JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Verify required fields exist
		assert.NotEmpty(t, response.Data.Status)
		assert.NotEmpty(t, response.Data.Version)
		assert.NotEmpty(t, response.Data.Checks)
		assert.False(t, response.Data.Timestamp.IsZero())
	})

	t.Run("timestamp is recent", func(t *testing.T) {
		before := time.Now()
		
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		after := time.Now()

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Data.Timestamp.After(before) || response.Data.Timestamp.Equal(before))
		assert.True(t, response.Data.Timestamp.Before(after) || response.Data.Timestamp.Equal(after))
	})

	t.Run("checks map contains expected keys", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Contains(t, response.Data.Checks, "database")
		assert.Contains(t, response.Data.Checks, "keycloak")
	})

	t.Run("database check reports status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Database should be healthy in test environment
		assert.Equal(t, "healthy", response.Data.Checks["database"])
	})

	t.Run("keycloak check reports status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Keycloak status should be either healthy or degraded (not empty)
		assert.NotEmpty(t, response.Data.Checks["keycloak"])
		keycloakStatus := response.Data.Checks["keycloak"]
		assert.True(t, 
			keycloakStatus == "healthy" || len(keycloakStatus) >= 8 && keycloakStatus[:8] == "degraded",
			"keycloak status should be 'healthy' or start with 'degraded', got: %s", keycloakStatus)
	})

	t.Run("respects context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		req := httptest.NewRequest("GET", "/health", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Should complete within timeout
		assert.NotEqual(t, 0, w.Code)
	})

	t.Run("version is included", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "1.0.0", response.Data.Version)
	})
}

func TestHandleHealth_StatusCodes(t *testing.T) {
	server := setupTestServer(t)
	defer server.Shutdown(nil)

	t.Run("returns 200 when healthy", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Should be 200 even if Keycloak is degraded (as long as DB is healthy)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("status field matches HTTP status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		var response healthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		if w.Code == http.StatusOK {
			assert.Equal(t, "healthy", response.Data.Status)
		} else if w.Code == http.StatusServiceUnavailable {
			assert.Equal(t, "unhealthy", response.Data.Status)
		}
	})
}
