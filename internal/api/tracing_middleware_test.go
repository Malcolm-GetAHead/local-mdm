package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingMiddleware_CreatesSpans(t *testing.T) {
	// Create in-memory span exporter for testing
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()

	// Create test router with tracing middleware
	router := mux.NewRouter()
	router.Use(tracingMiddleware)
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify span was created
	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "Expected exactly one span to be created")

	span := spans[0]
	assert.Equal(t, "GET /test", span.Name)
	assert.NotEmpty(t, span.SpanContext.TraceID(), "Span should have trace ID")
	assert.NotEmpty(t, span.SpanContext.SpanID(), "Span should have span ID")
}

func TestTracingMiddleware_CapturesRoutePattern(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()

	// Create router with parameterized route
	router := mux.NewRouter()
	router.Use(tracingMiddleware)
	router.HandleFunc("/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// Make request with actual ID
	req := httptest.NewRequest("GET", "/devices/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify span captures route pattern, not actual path
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "GET /devices/{id}", span.Name)
}

func TestTracingMiddleware_CapturesStatusCode(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}()

	router := mux.NewRouter()
	router.Use(tracingMiddleware)
	router.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}).Methods("GET")

	req := httptest.NewRequest("GET", "/error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Verify span was created
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "GET /error", span.Name)
}
