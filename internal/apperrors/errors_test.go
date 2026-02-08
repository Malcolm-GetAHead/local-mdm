package apperrors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError(t *testing.T) {
	t.Run("Error returns message with internal error", func(t *testing.T) {
		internal := errors.New("database connection failed")
		err := NewInternal(internal)
		
		assert.Contains(t, err.Error(), "An internal error occurred")
		assert.Contains(t, err.Error(), "database connection failed")
	})

	t.Run("Error returns just message when no internal error", func(t *testing.T) {
		err := NewNotFound("device")
		assert.Equal(t, "device not found", err.Error())
	})

	t.Run("Unwrap returns internal error", func(t *testing.T) {
		internal := errors.New("test error")
		err := NewInternal(internal)
		
		assert.Equal(t, internal, errors.Unwrap(err))
	})
}

func TestNewBadRequest(t *testing.T) {
	internal := errors.New("invalid JSON")
	err := NewBadRequest("Invalid request format", internal)
	
	assert.Equal(t, ErrCodeBadRequest, err.Code)
	assert.Equal(t, "Invalid request format", err.Message)
	assert.Equal(t, internal, err.Internal)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
}

func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("Authentication required")
	
	assert.Equal(t, ErrCodeUnauthorized, err.Code)
	assert.Equal(t, "Authentication required", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
}

func TestNewForbidden(t *testing.T) {
	err := NewForbidden("Insufficient permissions")
	
	assert.Equal(t, ErrCodeForbidden, err.Code)
	assert.Equal(t, "Insufficient permissions", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
}

func TestNewNotFound(t *testing.T) {
	err := NewNotFound("device")
	
	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Equal(t, "device not found", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
}

func TestNewConflict(t *testing.T) {
	internal := errors.New("unique constraint violation")
	err := NewConflict("Resource already exists", internal)
	
	assert.Equal(t, ErrCodeConflict, err.Code)
	assert.Equal(t, "Resource already exists", err.Message)
	assert.Equal(t, internal, err.Internal)
	assert.Equal(t, http.StatusConflict, err.StatusCode)
}

func TestNewValidation(t *testing.T) {
	err := NewValidation("Email is required")
	
	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "Email is required", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
}

func TestNewRateLimit(t *testing.T) {
	err := NewRateLimit("Too many requests")
	
	assert.Equal(t, ErrCodeRateLimit, err.Code)
	assert.Equal(t, "Too many requests", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode)
}

func TestNewInternal(t *testing.T) {
	internal := errors.New("database query failed: connection timeout")
	err := NewInternal(internal)
	
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, "An internal error occurred", err.Message)
	assert.Equal(t, internal, err.Internal)
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	
	// Verify internal details are NOT in the user-facing message
	assert.NotContains(t, err.Message, "database")
	assert.NotContains(t, err.Message, "connection timeout")
}

func TestNewServiceUnavailable(t *testing.T) {
	internal := errors.New("keycloak connection failed")
	err := NewServiceUnavailable("Authentication service unavailable", internal)
	
	assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
	assert.Equal(t, "Authentication service unavailable", err.Message)
	assert.Equal(t, internal, err.Internal)
	assert.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
}

func TestNewTimeout(t *testing.T) {
	err := NewTimeout("Request timeout")
	
	assert.Equal(t, ErrCodeTimeout, err.Code)
	assert.Equal(t, "Request timeout", err.Message)
	assert.Nil(t, err.Internal)
	assert.Equal(t, http.StatusGatewayTimeout, err.StatusCode)
}

func TestIsAppError(t *testing.T) {
	t.Run("returns true for AppError", func(t *testing.T) {
		err := NewNotFound("device")
		assert.True(t, IsAppError(err))
	})

	t.Run("returns false for standard error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.False(t, IsAppError(err))
	})

	t.Run("returns true for wrapped AppError", func(t *testing.T) {
		appErr := NewNotFound("device")
		wrapped := errors.Join(appErr, errors.New("additional context"))
		assert.True(t, IsAppError(wrapped))
	})
}

func TestAsAppError(t *testing.T) {
	t.Run("returns nil for nil error", func(t *testing.T) {
		result := AsAppError(nil)
		assert.Nil(t, result)
	})

	t.Run("returns same AppError", func(t *testing.T) {
		original := NewNotFound("device")
		result := AsAppError(original)
		assert.Equal(t, original, result)
	})

	t.Run("wraps standard error as internal error", func(t *testing.T) {
		original := errors.New("database connection failed")
		result := AsAppError(original)
		
		require.NotNil(t, result)
		assert.Equal(t, ErrCodeInternal, result.Code)
		assert.Equal(t, "An internal error occurred", result.Message)
		assert.Equal(t, original, result.Internal)
		assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	})

	t.Run("extracts AppError from wrapped error", func(t *testing.T) {
		appErr := NewNotFound("device")
		wrapped := errors.Join(appErr, errors.New("additional context"))
		result := AsAppError(wrapped)
		
		require.NotNil(t, result)
		assert.Equal(t, ErrCodeNotFound, result.Code)
	})
}

func TestErrorSanitization(t *testing.T) {
	t.Run("internal database errors are sanitized", func(t *testing.T) {
		// Simulate a database error with sensitive information
		internal := errors.New("pq: relation \"devices\" does not exist at line 42 in /internal/repository/device.go")
		err := NewInternal(internal)
		
		// User-facing message should be generic
		assert.Equal(t, "An internal error occurred", err.Message)
		assert.NotContains(t, err.Message, "pq:")
		assert.NotContains(t, err.Message, "relation")
		assert.NotContains(t, err.Message, "/internal/")
		
		// But internal error should have full details
		assert.Contains(t, err.Internal.Error(), "pq:")
		assert.Contains(t, err.Internal.Error(), "relation")
	})

	t.Run("file path errors are sanitized", func(t *testing.T) {
		internal := errors.New("open /etc/local-mdm/config.yaml: permission denied")
		err := NewInternal(internal)
		
		// User-facing message should be generic
		assert.Equal(t, "An internal error occurred", err.Message)
		assert.NotContains(t, err.Message, "/etc/")
		assert.NotContains(t, err.Message, "permission denied")
		
		// But internal error should have full details
		assert.Contains(t, err.Internal.Error(), "/etc/")
	})

	t.Run("stack traces are not exposed", func(t *testing.T) {
		internal := errors.New("panic: runtime error: invalid memory address or nil pointer dereference")
		err := NewInternal(internal)
		
		// User-facing message should be generic
		assert.Equal(t, "An internal error occurred", err.Message)
		assert.NotContains(t, err.Message, "panic")
		assert.NotContains(t, err.Message, "nil pointer")
	})
}

func TestErrorChaining(t *testing.T) {
	t.Run("errors.Is works with AppError", func(t *testing.T) {
		original := errors.New("original error")
		appErr := NewInternal(original)
		
		assert.True(t, errors.Is(appErr, original))
	})

	t.Run("errors.As works with AppError", func(t *testing.T) {
		appErr := NewNotFound("device")
		wrapped := errors.Join(appErr, errors.New("context"))
		
		var extracted *AppError
		assert.True(t, errors.As(wrapped, &extracted))
		assert.Equal(t, appErr.Code, extracted.Code)
	})
}
