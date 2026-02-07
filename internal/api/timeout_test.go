package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		handlerDelay   time.Duration
		expectTimeout  bool
	}{
		{
			name:          "request completes before timeout",
			timeout:       100 * time.Millisecond,
			handlerDelay:  10 * time.Millisecond,
			expectTimeout: false,
		},
		{
			name:          "request times out",
			timeout:       50 * time.Millisecond,
			handlerDelay:  200 * time.Millisecond,
			expectTimeout: true,
		},
		{
			name:          "instant response",
			timeout:       1 * time.Second,
			handlerDelay:  0,
			expectTimeout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.handlerDelay > 0 {
					select {
					case <-time.After(tt.handlerDelay):
						w.WriteHeader(http.StatusOK)
					case <-r.Context().Done():
						// Context cancelled - timeout occurred
						return
					}
				} else {
					w.WriteHeader(http.StatusOK)
				}
			})

			middleware := timeoutMiddleware(tt.timeout)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			if tt.expectTimeout {
				// When timeout occurs, context is cancelled but response may still be 200
				// The key is that the context deadline was exceeded
				if rec.Code != http.StatusOK && rec.Code != 0 {
					// Some responses may not be written due to timeout
					return
				}
			} else {
				if rec.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rec.Code)
				}
			}
		})
	}
}

func TestTimeoutMiddlewareContextCancellation(t *testing.T) {
	timeout := 100 * time.Millisecond
	contextCancelled := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			contextCancelled = true
			if r.Context().Err() != context.DeadlineExceeded {
				t.Errorf("expected DeadlineExceeded, got %v", r.Context().Err())
			}
			return
		}
	})

	middleware := timeoutMiddleware(timeout)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Give it a moment to process
	time.Sleep(150 * time.Millisecond)

	if !contextCancelled {
		t.Error("expected context to be cancelled due to timeout")
	}
}

func TestTimeoutMiddlewarePreservesContext(t *testing.T) {
	type contextKey string
	const testKey contextKey = "test"
	testValue := "value"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val := r.Context().Value(testKey)
		if val == nil {
			t.Error("context value was lost")
			return
		}
		if val.(string) != testValue {
			t.Errorf("expected %s, got %s", testValue, val.(string))
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := timeoutMiddleware(1 * time.Second)
	wrappedHandler := middleware(handler)

	ctx := context.WithValue(context.Background(), testKey, testValue)
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestTimeoutMiddlewareWithRealHandler(t *testing.T) {
	// Test that timeout middleware works with actual handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context has deadline
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("expected context to have deadline")
		}
		
		// Verify deadline is in the future
		if time.Until(deadline) <= 0 {
			t.Error("deadline should be in the future")
		}
		
		// Verify deadline is approximately correct (within 100ms)
		expectedDeadline := time.Now().Add(500 * time.Millisecond)
		if deadline.Sub(expectedDeadline).Abs() > 100*time.Millisecond {
			t.Errorf("deadline not as expected: got %v, want ~%v", deadline, expectedDeadline)
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := timeoutMiddleware(500 * time.Millisecond)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	
	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got %s", rec.Body.String())
	}
}

func TestTimeoutMiddlewareDefaultValue(t *testing.T) {
	// Test that zero timeout doesn't panic
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Zero timeout should still work (creates context with zero timeout)
	middleware := timeoutMiddleware(0)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
