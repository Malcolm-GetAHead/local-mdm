package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleError(t *testing.T) {
	t.Run("handles AppError correctly", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		err := apperrors.NewNotFound("device")
		HandleError(w, req, err, logger)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response Response
		require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
		assert.Equal(t, "not_found", response.Error.Code)
		assert.Equal(t, "device not found", response.Error.Message)
	})

	t.Run("sanitizes internal errors", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		// Create error with sensitive internal details
		internal := errors.New("pq: relation \"devices\" does not exist at /internal/repository/device.go:42")
		err := apperrors.NewInternal(internal)
		HandleError(w, req, err, logger)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response Response
		require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
		
		// User-facing message should be sanitized
		assert.Equal(t, "internal_error", response.Error.Code)
		assert.Equal(t, "An internal error occurred", response.Error.Message)
		assert.NotContains(t, response.Error.Message, "pq:")
		assert.NotContains(t, response.Error.Message, "relation")
		assert.NotContains(t, response.Error.Message, "/internal/")

		// But internal details should be logged
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "pq:")
		assert.Contains(t, logOutput, "relation")
	})

	t.Run("logs internal error details", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/devices", nil)

		internal := errors.New("database connection timeout")
		err := apperrors.NewInternal(internal)
		HandleError(w, req, err, logger)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "Request failed")
		assert.Contains(t, logOutput, "database connection timeout")
		assert.Contains(t, logOutput, "/api/devices")
		assert.Contains(t, logOutput, "GET")
		assert.Contains(t, logOutput, "internal_error")
	})

	t.Run("does not log when no internal error", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		// Error with no internal details
		err := apperrors.NewNotFound("device")
		HandleError(w, req, err, logger)

		// Should not log anything (no internal error)
		logOutput := logBuf.String()
		assert.Empty(t, logOutput)
	})

	t.Run("wraps standard errors as internal errors", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		// Standard error (not AppError)
		err := errors.New("unexpected error")
		HandleError(w, req, err, logger)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response Response
		require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
		assert.Equal(t, "internal_error", response.Error.Code)
		assert.Equal(t, "An internal error occurred", response.Error.Message)

		// Internal details should be logged
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "unexpected error")
	})

	t.Run("handles nil error gracefully", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		HandleError(w, req, nil, logger)

		// Should not write anything
		assert.Equal(t, 200, w.Code) // Default status
		assert.Empty(t, w.Body.String())
	})

	t.Run("includes request ID in logs", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Add request ID to context using the same key as the middleware
		ctx := req.Context()
		ctx = context.WithValue(ctx, requestIDKey, "test-request-123")
		req = req.WithContext(ctx)

		internal := errors.New("test error")
		err := apperrors.NewInternal(internal)
		HandleError(w, req, err, logger)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "test-request-123")
		assert.Contains(t, logOutput, "request_id")
	})
}

func TestErrorHandlerIntegration(t *testing.T) {
	t.Run("full request lifecycle with error", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		// Simulate a handler that returns an error
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate database error
			dbErr := errors.New("pq: connection refused to database at 192.168.1.100:5432")
			err := apperrors.NewInternal(dbErr)
			HandleError(w, r, err, logger)
		})

		req := httptest.NewRequest("POST", "/api/devices", nil)
		ctx := context.WithValue(req.Context(), requestIDKey, "integration-test-id")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response Response
		require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
		
		// Client sees sanitized error
		assert.Equal(t, "internal_error", response.Error.Code)
		assert.Equal(t, "An internal error occurred", response.Error.Message)
		assert.NotContains(t, response.Error.Message, "192.168.1.100")
		assert.NotContains(t, response.Error.Message, "connection refused")

		// Logs contain full details
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "192.168.1.100")
		assert.Contains(t, logOutput, "connection refused")
		assert.Contains(t, logOutput, "integration-test-id")
		assert.Contains(t, logOutput, "/api/devices")
		assert.Contains(t, logOutput, "POST")
	})
}
