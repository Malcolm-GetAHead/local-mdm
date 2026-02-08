package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiter_IPBasedLimit(t *testing.T) {
	limiter := newAuthRateLimiter(3, time.Second, 5, 5*time.Second)
	defer limiter.Stop()

	ip := "192.168.1.100"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.ipLimiter.allow(ip) {
			t.Errorf("IP request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.ipLimiter.allow(ip) {
		t.Error("4th IP request should be denied")
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !limiter.ipLimiter.allow(ip) {
		t.Error("IP request after window should be allowed")
	}
}

func TestAuthRateLimiter_AccountBasedLimit(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 3, time.Second)
	defer limiter.Stop()

	account := "user@example.com"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.accountLimiter.allow(account) {
			t.Errorf("Account request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.accountLimiter.allow(account) {
		t.Error("4th account request should be denied")
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !limiter.accountLimiter.allow(account) {
		t.Error("Account request after window should be allowed")
	}
}

func TestAuthRateLimiter_IndependentLimits(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Second, 2, time.Second)
	defer limiter.Stop()

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"
	account1 := "user1@example.com"
	account2 := "user2@example.com"

	// Different IPs should have independent limits
	if !limiter.ipLimiter.allow(ip1) {
		t.Error("IP1 first request should be allowed")
	}
	if !limiter.ipLimiter.allow(ip2) {
		t.Error("IP2 first request should be allowed")
	}

	// Different accounts should have independent limits
	if !limiter.accountLimiter.allow(account1) {
		t.Error("Account1 first request should be allowed")
	}
	if !limiter.accountLimiter.allow(account2) {
		t.Error("Account2 first request should be allowed")
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1, 192.0.2.1")
	req.RemoteAddr = "10.0.0.1:12345"

	ip := getClientIP(req)

	if ip != "203.0.113.1" {
		t.Errorf("expected IP 203.0.113.1, got %s", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	req.RemoteAddr = "10.0.0.1:12345"

	ip := getClientIP(req)

	if ip != "203.0.113.5" {
		t.Errorf("expected IP 203.0.113.5, got %s", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	ip := getClientIP(req)

	if ip != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", ip)
	}
}

func TestGetClientIP_Priority(t *testing.T) {
	// X-Forwarded-For should take priority over X-Real-IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	req.RemoteAddr = "10.0.0.1:12345"

	ip := getClientIP(req)

	if ip != "203.0.113.1" {
		t.Errorf("expected X-Forwarded-For IP 203.0.113.1, got %s", ip)
	}
}

func TestAuthRateLimitMiddleware_IPLimit(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Second, 10, time.Minute)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// First 2 requests should be allowed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d should be allowed, got status %d", i+1, w.Code)
		}
	}

	// 3rd request should be denied
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request should be denied, got status %d", w.Code)
	}

	// Check Retry-After header
	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be set")
	}
}

func TestAuthRateLimitMiddleware_AccountLimit(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	loginReq := map[string]string{
		"username": "test@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(loginReq)

	// First 2 requests should be allowed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d should be allowed, got status %d", i+1, w.Code)
		}
	}

	// 3rd request should be denied (account limit)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request should be denied, got status %d", w.Code)
	}

	// Check error message mentions account
	var response struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.NotNil(t, response.Error)
	message := response.Error.Message
	if !strings.Contains(strings.ToLower(message), "account") {
		t.Errorf("error message should mention account, got: %s", message)
	}
}

func TestAuthRateLimitMiddleware_DifferentAccounts(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	accounts := []string{"user1@example.com", "user2@example.com", "user3@example.com"}

	// Each account should have independent limits
	for _, account := range accounts {
		loginReq := map[string]string{
			"username": account,
			"password": "password123",
		}
		body, _ := json.Marshal(loginReq)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("account %s request %d should be allowed, got status %d", account, i+1, w.Code)
			}
		}
	}
}

func TestAuthRateLimitMiddleware_RefreshEndpoint(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Second, 10, time.Minute)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Refresh endpoint should only check IP limit, not account limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("refresh request %d should be allowed, got status %d", i+1, w.Code)
		}
	}

	// 3rd request should be denied (IP limit)
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("3rd refresh request should be denied, got status %d", w.Code)
	}
}

func TestAuthRateLimitMiddleware_InvalidJSON(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 10, time.Minute)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Invalid JSON should still pass through (let handler deal with it)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("invalid json"))
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Should pass IP check and reach handler
	if w.Code != http.StatusOK {
		t.Errorf("invalid JSON should pass rate limit check, got status %d", w.Code)
	}
}

func TestAuthRateLimitMiddleware_EmptyUsername(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	loginReq := map[string]string{
		"username": "",
		"password": "password123",
	}
	body, _ := json.Marshal(loginReq)

	// Empty username should only check IP limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d with empty username should be allowed (IP limit not reached), got status %d", i+1, w.Code)
		}
	}
}

func TestAuthRateLimitMiddleware_UsernameNormalization(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Different case variations of same username should share limit
	usernames := []string{"Test@Example.com", "test@example.com", "TEST@EXAMPLE.COM"}

	for i, username := range usernames {
		loginReq := map[string]string{
			"username": username,
			"password": "password123",
		}
		body, _ := json.Marshal(loginReq)

		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", i+1) // Different IPs
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if i < 2 {
			if w.Code != http.StatusOK {
				t.Errorf("request %d should be allowed, got status %d", i+1, w.Code)
			}
		} else {
			// 3rd variation should be denied (same normalized username)
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("request %d should be denied (account limit), got status %d", i+1, w.Code)
			}
		}
	}
}

func TestAuthRateLimitMiddleware_ConcurrentRequests(t *testing.T) {
	limiter := newAuthRateLimiter(50, time.Second, 50, time.Second)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Launch 100 concurrent requests from same IP
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			loginReq := map[string]string{
				"username": fmt.Sprintf("user%d@example.com", id),
				"password": "password123",
			}
			body, _ := json.Marshal(loginReq)

			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Should allow exactly 50 requests (IP limit)
	if successCount != 50 {
		t.Errorf("expected 50 successful requests, got %d", successCount)
	}
}

func TestAuthRateLimitMiddleware_DifferentIPs(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Second, 10, time.Minute)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Different IPs should have independent limits
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", i)
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request from IP %d should be allowed, got status %d", i, w.Code)
		}
	}
}

func TestLoginSuccessTracker_RecordSuccess(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	tracker := newLoginSuccessTracker(limiter)

	username := "test@example.com"
	ip := "192.168.1.1"

	// Exhaust account limit
	limiter.accountLimiter.allow(username)
	limiter.accountLimiter.allow(username)

	// Verify limit reached
	if limiter.accountLimiter.allow(username) {
		t.Error("account limit should be reached")
	}

	// Record successful login
	tracker.RecordSuccess(username, ip)

	// Should be able to login again
	if !limiter.accountLimiter.allow(username) {
		t.Error("account limit should be reset after successful login")
	}
}

func TestLoginSuccessTracker_UsernameNormalization(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 2, time.Second)
	defer limiter.Stop()

	tracker := newLoginSuccessTracker(limiter)

	// Exhaust limit with lowercase
	limiter.accountLimiter.allow("test@example.com")
	limiter.accountLimiter.allow("test@example.com")

	// Record success with uppercase
	tracker.RecordSuccess("TEST@EXAMPLE.COM", "192.168.1.1")

	// Should be reset (normalized)
	if !limiter.accountLimiter.allow("test@example.com") {
		t.Error("account limit should be reset regardless of case")
	}
}

func TestLoginSuccessTracker_IPNotReset(t *testing.T) {
	limiter := newAuthRateLimiter(2, time.Second, 10, time.Minute)
	defer limiter.Stop()

	tracker := newLoginSuccessTracker(limiter)

	ip := "192.168.1.1"
	username := "test@example.com"

	// Exhaust IP limit
	limiter.ipLimiter.allow(ip)
	limiter.ipLimiter.allow(ip)

	// Verify IP limit reached
	if limiter.ipLimiter.allow(ip) {
		t.Error("IP limit should be reached")
	}

	// Record successful login
	tracker.RecordSuccess(username, ip)

	// IP limit should NOT be reset
	if limiter.ipLimiter.allow(ip) {
		t.Error("IP limit should NOT be reset on successful login")
	}
}

func TestAuthRateLimitMiddleware_BodyPreserved(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 10, time.Minute)
	defer limiter.Stop()

	middleware := authRateLimitMiddleware(limiter)

	var receivedBody string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	loginReq := map[string]string{
		"username": "test@example.com",
		"password": "secret123",
	}
	expectedBody, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(expectedBody))
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if receivedBody != string(expectedBody) {
		t.Errorf("body not preserved: expected %s, got %s", string(expectedBody), receivedBody)
	}
}

func TestAuthRateLimiter_Stop(t *testing.T) {
	limiter := newAuthRateLimiter(10, time.Minute, 10, time.Minute)

	// Add some entries
	limiter.ipLimiter.allow("192.168.1.1")
	limiter.accountLimiter.allow("test@example.com")

	// Stop should be safe to call
	limiter.Stop()

	// Calling Stop again should be safe (idempotent)
	limiter.Stop()

	// Limiter should still work after stop
	if !limiter.ipLimiter.allow("192.168.1.2") {
		t.Error("limiter should still work after stop")
	}
}
