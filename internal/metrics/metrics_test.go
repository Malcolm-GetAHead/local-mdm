package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates metrics without DB", func(t *testing.T) {
		m := New(nil)
		require.NotNil(t, m)
		assert.NotNil(t, m.HTTPRequestsTotal)
		assert.NotNil(t, m.HTTPRequestDuration)
		assert.NotNil(t, m.HTTPActiveRequests)
		assert.NotNil(t, m.EnrollmentsTotal)
		assert.NotNil(t, m.CommandsQueued)
		assert.NotNil(t, m.CommandsPending)
		assert.Nil(t, m.DBOpenConnections) // no DB provided
	})
}

func TestMetrics_Handler(t *testing.T) {
	m := New(nil)

	// Record some metrics
	m.HTTPRequestsTotal.WithLabelValues("GET", "/health", "200").Inc()
	m.HTTPRequestDuration.WithLabelValues("GET", "/health").Observe(0.05)
	m.EnrollmentsTotal.WithLabelValues("windows", "complete").Inc()
	m.CommandsQueued.WithLabelValues("lock").Inc()
	m.CommandsPending.Set(3)

	// Scrape the metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "enrollments_total")
	assert.Contains(t, body, "commands_queued_total")
	assert.Contains(t, body, "commands_pending")
	assert.Contains(t, body, `method="GET"`)
	assert.Contains(t, body, `platform="windows"`)
}

func TestMetrics_Middleware(t *testing.T) {
	m := New(nil)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify metrics were recorded — scrape and check
	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()

	assert.Contains(t, body, `http_requests_total{method="GET"`)
	assert.Contains(t, body, `status="200"`)
	assert.Contains(t, body, "http_request_duration_seconds")
}

func TestMetrics_Middleware_RecordsStatusCode(t *testing.T) {
	m := New(nil)

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/missing", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))

	assert.Contains(t, scrape.Body.String(), `status="404"`)
}

func TestNewServer(t *testing.T) {
	m := New(nil)

	t.Run("defaults to localhost:9090", func(t *testing.T) {
		s := NewServer("", 0, m, nil)
		assert.NotNil(t, s)
		assert.Equal(t, "127.0.0.1:9090", s.server.Addr)
	})

	t.Run("uses custom host and port", func(t *testing.T) {
		s := NewServer("0.0.0.0", 9191, m, nil)
		assert.Equal(t, "0.0.0.0:9191", s.server.Addr)
	})
}

func TestMetrics_EnrollmentCounters(t *testing.T) {
	m := New(nil)

	m.EnrollmentsTotal.WithLabelValues("macos", "profile_generated").Inc()
	m.EnrollmentsTotal.WithLabelValues("macos", "profile_generated").Inc()
	m.EnrollmentsTotal.WithLabelValues("windows", "complete").Inc()
	m.EnrollmentsTotal.WithLabelValues("android", "token_created").Inc()

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()

	// Count occurrences of each platform
	assert.True(t, strings.Contains(body, `platform="macos"`))
	assert.True(t, strings.Contains(body, `platform="windows"`))
	assert.True(t, strings.Contains(body, `platform="android"`))
}

func TestNew_WithDB(t *testing.T) {
	db, err := sql.Open("postgres", "")
	require.NoError(t, err)
	defer db.Close()

	m := New(db)
	require.NotNil(t, m)
	assert.NotNil(t, m.DBOpenConnections)
	assert.NotNil(t, m.DBIdleConnections)
	assert.NotNil(t, m.DBWaitCount)

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()
	assert.Contains(t, body, "db_open_connections")
	assert.Contains(t, body, "db_idle_connections")
	assert.Contains(t, body, "db_wait_count_total")
}

func TestMiddleware_MuxRouteTemplate(t *testing.T) {
	m := New(nil)

	router := mux.NewRouter()
	router.Handle("/api/v1/devices/{id}", m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/devices/abc-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
	body := scrape.Body.String()

	assert.Contains(t, body, `path="/api/v1/devices/{id}"`)
	assert.NotContains(t, body, `path="/api/v1/devices/abc-123"`)
}

func TestServer_StartAndShutdown(t *testing.T) {
	m := New(nil)

	// Get a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	logger := slog.Default()
	s := NewServer("127.0.0.1", port, m, logger)
	s.Start()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
}
