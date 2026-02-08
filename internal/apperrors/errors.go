package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode represents a machine-readable error code
type ErrorCode string

const (
	// Client errors (4xx)
	ErrCodeBadRequest       ErrorCode = "bad_request"
	ErrCodeUnauthorized     ErrorCode = "unauthorized"
	ErrCodeForbidden        ErrorCode = "forbidden"
	ErrCodeNotFound         ErrorCode = "not_found"
	ErrCodeConflict         ErrorCode = "conflict"
	ErrCodeValidation       ErrorCode = "validation_failed"
	ErrCodeRateLimit        ErrorCode = "rate_limit_exceeded"
	ErrCodeRequestTooLarge  ErrorCode = "request_too_large"
	
	// Server errors (5xx)
	ErrCodeInternal         ErrorCode = "internal_error"
	ErrCodeServiceUnavailable ErrorCode = "service_unavailable"
	ErrCodeTimeout          ErrorCode = "timeout"
)

// AppError represents an application error with both internal and external representations
type AppError struct {
	// Code is the machine-readable error code
	Code ErrorCode
	
	// Message is the user-facing error message (sanitized)
	Message string
	
	// Internal is the internal error with full details (never sent to client)
	Internal error
	
	// StatusCode is the HTTP status code
	StatusCode int
}

func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Internal
}

// NewBadRequest creates a bad request error
func NewBadRequest(message string, internal error) *AppError {
	return &AppError{
		Code:       ErrCodeBadRequest,
		Message:    message,
		Internal:   internal,
		StatusCode: http.StatusBadRequest,
	}
}

// NewUnauthorized creates an unauthorized error
func NewUnauthorized(message string) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorized,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewForbidden creates a forbidden error
func NewForbidden(message string) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewNotFound creates a not found error
func NewNotFound(resourceType string) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resourceType),
		StatusCode: http.StatusNotFound,
	}
}

// NewConflict creates a conflict error
func NewConflict(message string, internal error) *AppError {
	return &AppError{
		Code:       ErrCodeConflict,
		Message:    message,
		Internal:   internal,
		StatusCode: http.StatusConflict,
	}
}

// NewValidation creates a validation error
func NewValidation(message string) *AppError {
	return &AppError{
		Code:       ErrCodeValidation,
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// NewRateLimit creates a rate limit error
func NewRateLimit(message string) *AppError {
	return &AppError{
		Code:       ErrCodeRateLimit,
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
	}
}

// NewInternal creates an internal server error
func NewInternal(internal error) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		Message:    "An internal error occurred",
		Internal:   internal,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewServiceUnavailable creates a service unavailable error
func NewServiceUnavailable(message string, internal error) *AppError {
	return &AppError{
		Code:       ErrCodeServiceUnavailable,
		Message:    message,
		Internal:   internal,
		StatusCode: http.StatusServiceUnavailable,
	}
}

// NewTimeout creates a timeout error
func NewTimeout(message string) *AppError {
	return &AppError{
		Code:       ErrCodeTimeout,
		Message:    message,
		StatusCode: http.StatusGatewayTimeout,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// AsAppError converts an error to an AppError, or wraps it if it's not already one
func AsAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	
	// Wrap unknown errors as internal errors
	return NewInternal(err)
}
