package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserFromContext_Success tests retrieving user from context successfully
func TestUserFromContext_Success(t *testing.T) {
	user := &auth.AuthUser{
		ID:    "test-user-id",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	ctx := auth.WithUser(context.Background(), user)

	retrievedUser, err := auth.UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.Roles, retrievedUser.Roles)
}

// TestUserFromContext_NoUser tests error when no user in context
func TestUserFromContext_NoUser(t *testing.T) {
	ctx := context.Background()

	user, err := auth.UserFromContext(ctx)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "no user in context")
}

// TestUserFromContext_NilUser tests error when user is nil
func TestUserFromContext_NilUser(t *testing.T) {
	ctx := auth.WithUser(context.Background(), nil)

	user, err := auth.UserFromContext(ctx)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "no user in context")
}

// TestHandlerWithProperErrorHandling demonstrates correct handler pattern
func TestHandlerWithProperErrorHandling(t *testing.T) {
	// This is the CORRECT pattern for handlers
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.UserFromContext(r.Context())
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello " + user.Email))
	})

	t.Run("with user in context", func(t *testing.T) {
		user := &auth.AuthUser{
			ID:    "test-id",
			Email: "test@example.com",
			Roles: []string{"admin"},
		}

		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(auth.WithUser(req.Context(), user))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Hello test@example.com")
	})

	t.Run("without user in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestConcurrentContextAccess tests thread-safety of context operations
func TestConcurrentContextAccess(t *testing.T) {
	user := &auth.AuthUser{
		ID:    "test-id",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	ctx := auth.WithUser(context.Background(), user)

	// Simulate concurrent access from multiple goroutines
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				retrievedUser, err := auth.UserFromContext(ctx)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if retrievedUser.Email != user.Email {
					t.Errorf("Expected email %s, got %s", user.Email, retrievedUser.Email)
					return
				}
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestMultipleContextLayers tests context with multiple values
func TestMultipleContextLayers(t *testing.T) {
	user := &auth.AuthUser{
		ID:    "test-id",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	// Create context with user
	ctx := auth.WithUser(context.Background(), user)

	// Add more context values
	type requestIDKey struct{}
	ctx = context.WithValue(ctx, requestIDKey{}, "request-123")

	// User should still be retrievable
	retrievedUser, err := auth.UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, user.Email, retrievedUser.Email)

	// Other context values should also be retrievable
	requestID := ctx.Value(requestIDKey{})
	assert.Equal(t, "request-123", requestID)
}

// TestContextCancellation tests that user retrieval works with cancelled context
func TestContextCancellation(t *testing.T) {
	user := &auth.AuthUser{
		ID:    "test-id",
		Email: "test@example.com",
		Roles: []string{"admin"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = auth.WithUser(ctx, user)

	// User should be retrievable before cancellation
	retrievedUser, err := auth.UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, user.Email, retrievedUser.Email)

	// Cancel context
	cancel()

	// User should still be retrievable (context cancellation doesn't affect values)
	retrievedUser, err = auth.UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, user.Email, retrievedUser.Email)
}

// TestUserFromContext_EdgeCases tests various edge cases
func TestUserFromContext_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() context.Context
		wantErr bool
	}{
		{
			name: "empty user struct",
			setup: func() context.Context {
				return auth.WithUser(context.Background(), &auth.AuthUser{})
			},
			wantErr: false, // Empty user is still a valid user
		},
		{
			name: "user with empty roles",
			setup: func() context.Context {
				return auth.WithUser(context.Background(), &auth.AuthUser{
					ID:    "test-id",
					Email: "test@example.com",
					Roles: []string{},
				})
			},
			wantErr: false,
		},
		{
			name: "user with nil roles",
			setup: func() context.Context {
				return auth.WithUser(context.Background(), &auth.AuthUser{
					ID:    "test-id",
					Email: "test@example.com",
					Roles: nil,
				})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			user, err := auth.UserFromContext(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
			}
		})
	}
}

// TestHandlerErrorHandlingPatterns demonstrates various error handling patterns
func TestHandlerErrorHandlingPatterns(t *testing.T) {
	t.Run("pattern 1: early return on error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.UserFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			w.Write([]byte(user.Email))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("pattern 2: structured error response", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.UserFromContext(r.Context())
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized","message":"Authentication required"}`))
				return
			}
			w.Write([]byte(user.Email))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized")
	})

	t.Run("pattern 3: logging on error", func(t *testing.T) {
		var loggedError error
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.UserFromContext(r.Context())
			if err != nil {
				loggedError = err // In real code, use proper logger
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			w.Write([]byte(user.Email))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Error(t, loggedError)
		assert.Contains(t, loggedError.Error(), "no user in context")
	})
}

// TestNoPanicInHandlers ensures handlers never panic
func TestNoPanicInHandlers(t *testing.T) {
	// This test verifies that proper error handling prevents panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORRECT: Use UserFromContext with error handling
		user, err := auth.UserFromContext(r.Context())
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Safe to use user here
		w.Write([]byte(user.Email))
	})

	// Test should not panic even without user in context
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// This should not panic
	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
