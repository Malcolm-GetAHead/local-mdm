package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPAllowlistMiddleware_AllowedIP(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.0/24"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestIPAllowlistMiddleware_BlockedIP(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.0/24"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "ip_not_allowed")
}

func TestIPAllowlistMiddleware_SingleIP(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.100"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exact match should pass
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Different IP should fail
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.101:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPAllowlistMiddleware_MultipleCIDRs(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{
		"192.168.1.0/24",
		"10.0.0.0/8",
		"172.16.0.0/12",
	})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		ip       string
		expected int
	}{
		{"192.168.1.50:12345", http.StatusOK},
		{"10.5.10.20:12345", http.StatusOK},
		{"172.20.1.1:12345", http.StatusOK},
		{"8.8.8.8:12345", http.StatusForbidden},
		{"203.0.113.1:12345", http.StatusForbidden},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = tc.ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, tc.expected, rec.Code, "IP: %s", tc.ip)
	}
}

func TestIPAllowlistMiddleware_XForwardedFor(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.0/24"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// X-Forwarded-For should be used
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345" // This would be blocked
	req.Header.Set("X-Forwarded-For", "192.168.1.100") // But this is allowed
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_XForwardedFor_MultipleIPs(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.0/24"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// X-Forwarded-For with multiple IPs (leftmost is client)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.2, 10.0.0.3")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_XRealIP(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"192.168.1.0/24"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// X-Real-IP should be used
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "192.168.1.100")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_EmptyAllowlist(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Empty allowlist should allow all (fail open)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_InvalidCIDR(t *testing.T) {
	// Invalid CIDR should be skipped, not crash
	middleware := ipAllowlistMiddleware([]string{
		"192.168.1.0/24",
		"invalid-cidr",
		"10.0.0.0/8",
	})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Valid IPs should still work
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_IPv6(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"2001:db8::/32"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IPv6 in range
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// IPv6 out of range
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[2001:db9::1]:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPAllowlistMiddleware_IPv6_SingleIP(t *testing.T) {
	middleware := ipAllowlistMiddleware([]string{"2001:db8::1"})
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exact IPv6 match
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Different IPv6
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[2001:db8::2]:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
