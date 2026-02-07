package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	limiter := newRateLimiter(3, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond) // Let cleanup goroutine finish

	key := "test-key"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.allow(key) {
		t.Error("4th request should be denied")
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !limiter.allow(key) {
		t.Error("request after window should be allowed")
	}
}

func TestRateLimiter_MultipleKeys(t *testing.T) {
	limiter := newRateLimiter(2, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	// Different keys should have independent limits
	if !limiter.allow("key1") {
		t.Error("key1 first request should be allowed")
	}
	if !limiter.allow("key2") {
		t.Error("key2 first request should be allowed")
	}
	if !limiter.allow("key1") {
		t.Error("key1 second request should be allowed")
	}
	if !limiter.allow("key2") {
		t.Error("key2 second request should be allowed")
	}

	// Both should be at limit
	if limiter.allow("key1") {
		t.Error("key1 third request should be denied")
	}
	if limiter.allow("key2") {
		t.Error("key2 third request should be denied")
	}
}

func TestRateLimiter_LRUEviction(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Minute)
	limiter.maxSize = 3 // Set small max size for testing
	defer time.Sleep(100 * time.Millisecond)

	// Add 3 keys (at capacity)
	limiter.allow("key1")
	limiter.allow("key2")
	limiter.allow("key3")

	// Verify all 3 are tracked
	limiter.mu.RLock()
	if len(limiter.requests) != 3 {
		t.Errorf("expected 3 entries, got %d", len(limiter.requests))
	}
	limiter.mu.RUnlock()

	// Add 4th key - should evict oldest (key1)
	limiter.allow("key4")

	limiter.mu.RLock()
	if len(limiter.requests) != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", len(limiter.requests))
	}
	if _, exists := limiter.requests["key1"]; exists {
		t.Error("key1 should have been evicted")
	}
	if _, exists := limiter.requests["key4"]; !exists {
		t.Error("key4 should exist")
	}
	limiter.mu.RUnlock()
}

func TestRateLimiter_LRUOrdering(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Minute)
	limiter.maxSize = 3
	defer time.Sleep(100 * time.Millisecond)

	// Add 3 keys
	limiter.allow("key1")
	limiter.allow("key2")
	limiter.allow("key3")

	// Access key1 again (moves to front)
	limiter.allow("key1")

	// Add key4 - should evict key2 (oldest)
	limiter.allow("key4")

	limiter.mu.RLock()
	if _, exists := limiter.requests["key2"]; exists {
		t.Error("key2 should have been evicted (was oldest)")
	}
	if _, exists := limiter.requests["key1"]; !exists {
		t.Error("key1 should still exist (was accessed recently)")
	}
	limiter.mu.RUnlock()
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := newRateLimiter(5, 100*time.Millisecond)
	defer time.Sleep(100 * time.Millisecond)

	// Add some requests
	limiter.allow("key1")
	limiter.allow("key2")

	// Verify they exist
	limiter.mu.RLock()
	initialCount := len(limiter.requests)
	limiter.mu.RUnlock()

	if initialCount != 2 {
		t.Errorf("expected 2 entries, got %d", initialCount)
	}

	// Wait for entries to expire (2x window)
	time.Sleep(250 * time.Millisecond)

	// Run cleanup
	limiter.cleanup()

	// Verify they were removed
	limiter.mu.RLock()
	finalCount := len(limiter.requests)
	limiter.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", finalCount)
	}
}

func TestRateLimiter_CleanupPreservesRecent(t *testing.T) {
	limiter := newRateLimiter(5, 100*time.Millisecond)
	defer time.Sleep(100 * time.Millisecond)

	// Add old request
	limiter.allow("old-key")

	// Wait for it to age
	time.Sleep(250 * time.Millisecond)

	// Add new request
	limiter.allow("new-key")

	// Run cleanup
	limiter.cleanup()

	limiter.mu.RLock()
	_, oldExists := limiter.requests["old-key"]
	_, newExists := limiter.requests["new-key"]
	limiter.mu.RUnlock()

	if oldExists {
		t.Error("old-key should have been cleaned up")
	}
	if !newExists {
		t.Error("new-key should still exist")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := newRateLimiter(100, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	// Run concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				limiter.allow("concurrent-key")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have tracked requests
	limiter.mu.RLock()
	count := len(limiter.requests)
	limiter.mu.RUnlock()

	if count == 0 {
		t.Error("expected requests to be tracked")
	}
}

func TestRateLimiter_MaxSizeEnforced(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Minute)
	limiter.maxSize = 100
	defer time.Sleep(100 * time.Millisecond)

	// Add more than maxSize entries
	for i := 0; i < 150; i++ {
		limiter.allow(string(rune(i)))
	}

	limiter.mu.RLock()
	count := len(limiter.requests)
	limiter.mu.RUnlock()

	if count > limiter.maxSize {
		t.Errorf("expected at most %d entries, got %d", limiter.maxSize, count)
	}
}

func TestRateLimiter_EvictOldestEmptyList(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Minute)
	defer time.Sleep(100 * time.Millisecond)

	// Call evictOldest on empty limiter (should not panic)
	limiter.evictOldest()

	// Verify no panic and state is clean
	limiter.mu.RLock()
	count := len(limiter.requests)
	limiter.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 entries, got %d", count)
	}
}

func TestRateLimiter_CleanupGoroutine(t *testing.T) {
	// Create limiter with short window for faster test
	limiter := newRateLimiter(5, 50*time.Millisecond)

	// Add some entries
	limiter.allow("key1")
	limiter.allow("key2")

	// Verify they exist
	limiter.mu.RLock()
	initialCount := len(limiter.requests)
	limiter.mu.RUnlock()

	if initialCount != 2 {
		t.Errorf("expected 2 entries, got %d", initialCount)
	}

	// Wait for cleanup goroutine to run (>1 minute + 2x window)
	// Since we can't wait that long, we'll manually trigger cleanup
	time.Sleep(150 * time.Millisecond) // Wait for entries to expire

	// Manually trigger cleanup to test the goroutine's work
	limiter.cleanup()

	// Verify cleanup worked
	limiter.mu.RLock()
	finalCount := len(limiter.requests)
	limiter.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", finalCount)
	}
}

func TestRateLimiter_EdgeCaseZeroLimit(t *testing.T) {
	limiter := newRateLimiter(0, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	// With limit 0, all requests should be denied
	if limiter.allow("test-key") {
		t.Error("request should be denied with limit 0")
	}
}

func TestRateLimiter_EdgeCaseNegativeLimit(t *testing.T) {
	limiter := newRateLimiter(-1, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	// With negative limit, all requests should be denied
	if limiter.allow("test-key") {
		t.Error("request should be denied with negative limit")
	}
}

func TestRateLimiter_RapidEviction(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Minute)
	limiter.maxSize = 10
	defer time.Sleep(100 * time.Millisecond)

	// Rapidly add entries beyond capacity
	for i := 0; i < 50; i++ {
		limiter.allow(string(rune(i)))
	}

	limiter.mu.RLock()
	count := len(limiter.requests)
	lruLen := limiter.lru.Len()
	lruMapLen := len(limiter.lruMap)
	limiter.mu.RUnlock()

	// Verify all structures are consistent
	if count != lruLen {
		t.Errorf("requests map (%d) and LRU list (%d) out of sync", count, lruLen)
	}
	if count != lruMapLen {
		t.Errorf("requests map (%d) and LRU map (%d) out of sync", count, lruMapLen)
	}
	if count > limiter.maxSize {
		t.Errorf("expected at most %d entries, got %d", limiter.maxSize, count)
	}
}

func TestRateLimitMiddleware_Allow(t *testing.T) {
	limiter := newRateLimiter(3, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	middleware := rateLimitMiddleware(limiter)

	// Create a test handler that tracks if it was called
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		called = false
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if !called {
			t.Errorf("request %d should have been allowed", i+1)
		}
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddleware_Deny(t *testing.T) {
	limiter := newRateLimiter(2, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	middleware := rateLimitMiddleware(limiter)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First 2 requests allowed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}

	// 3rd request should be denied
	called = false
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if called {
		t.Error("handler should not have been called (rate limited)")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	limiter := newRateLimiter(1, 1*time.Second)
	defer time.Sleep(100 * time.Millisecond)

	middleware := rateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Different IPs should have independent limits
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}

	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request from %s should be allowed, got status %d", ip, w.Code)
		}
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Second)

	// Add some entries
	limiter.allow("key1")
	limiter.allow("key2")

	// Stop the limiter
	limiter.Stop()

	// Verify stopped flag is set
	limiter.mu.RLock()
	stopped := limiter.stopped
	limiter.mu.RUnlock()

	if !stopped {
		t.Error("limiter should be marked as stopped")
	}

	// Calling Stop again should be safe (idempotent)
	limiter.Stop()
}

func TestRateLimiter_StopGoroutine(t *testing.T) {
	limiter := newRateLimiter(5, 1*time.Second)

	// Stop immediately
	limiter.Stop()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	// Limiter should still work after stop
	if !limiter.allow("test-key") {
		t.Error("limiter should still allow requests after stop")
	}
}

func TestNewRateLimiterWithSize(t *testing.T) {
	customSize := 500
	limiter := newRateLimiterWithSize(10, 1*time.Minute, customSize)
	defer limiter.Stop()

	if limiter.maxSize != customSize {
		t.Errorf("expected maxSize %d, got %d", customSize, limiter.maxSize)
	}

	// Add entries up to custom size
	for i := 0; i < customSize+10; i++ {
		limiter.allow(string(rune(i)))
	}

	limiter.mu.RLock()
	count := len(limiter.requests)
	limiter.mu.RUnlock()

	if count > customSize {
		t.Errorf("expected at most %d entries, got %d", customSize, count)
	}
}

func TestRateLimiter_ConfigurableSize(t *testing.T) {
	tests := []struct {
		name    string
		maxSize int
		entries int
	}{
		{"small", 10, 20},
		{"medium", 100, 150},
		{"large", 1000, 1500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := newRateLimiterWithSize(5, 1*time.Minute, tt.maxSize)
			defer limiter.Stop()

			// Add more entries than maxSize
			for i := 0; i < tt.entries; i++ {
				limiter.allow(string(rune(i)))
			}

			limiter.mu.RLock()
			count := len(limiter.requests)
			limiter.mu.RUnlock()

			if count > tt.maxSize {
				t.Errorf("%s: expected at most %d entries, got %d", tt.name, tt.maxSize, count)
			}
		})
	}
}


