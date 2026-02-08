package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// authRateLimiter implements strict rate limiting for authentication endpoints
// with both IP-based and account-based tracking
type authRateLimiter struct {
	ipLimiter      *rateLimiter
	accountLimiter *rateLimiter
}

func newAuthRateLimiter(ipLimit int, ipWindow time.Duration, accountLimit int, accountWindow time.Duration) *authRateLimiter {
	return &authRateLimiter{
		ipLimiter:      newRateLimiter(ipLimit, ipWindow),
		accountLimiter: newRateLimiter(accountLimit, accountWindow),
	}
}

func (arl *authRateLimiter) Stop() {
	arl.ipLimiter.Stop()
	arl.accountLimiter.Stop()
}

// getClientIP extracts the real client IP from the request
// Handles IPv4, IPv6, X-Forwarded-For, and X-Real-IP headers
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	// Use SplitHostPort to handle both IPv4 and IPv6 with ports
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, return as-is (might be IP without port)
		return r.RemoteAddr
	}
	return host
}

// authRateLimitMiddleware creates middleware for authentication endpoints
func authRateLimitMiddleware(limiter *authRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			
			// Check IP-based rate limit first
			if !limiter.ipLimiter.allow(ip) {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(limiter.ipLimiter.window.Seconds())))
				respondError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", 
					"Too many authentication attempts from this IP. Please try again later.")
				return
			}
			
			// For login requests, also check account-based rate limit
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/login") {
				// Read and buffer the body to extract username
				body, err := io.ReadAll(r.Body)
				if err != nil {
					respondError(w, r, http.StatusBadRequest, "invalid_request", "Failed to read request body")
					return
				}
				r.Body.Close()
				
				// Parse username from request
				var req struct {
					Username string `json:"username"`
				}
				if err := json.Unmarshal(body, &req); err == nil && req.Username != "" {
					// Normalize username (lowercase for consistent tracking)
					username := strings.ToLower(strings.TrimSpace(req.Username))
					
					if !limiter.accountLimiter.allow(username) {
						w.Header().Set("Retry-After", fmt.Sprintf("%d", int(limiter.accountLimiter.window.Seconds())))
						respondError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", 
							"Too many failed login attempts for this account. Please try again later.")
						return
					}
				}
				
				// Restore body for downstream handlers
				r.Body = io.NopCloser(strings.NewReader(string(body)))
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// loginSuccessTracker tracks successful logins to reset rate limits
type loginSuccessTracker struct {
	limiter *authRateLimiter
	mu      sync.RWMutex
}

func newLoginSuccessTracker(limiter *authRateLimiter) *loginSuccessTracker {
	return &loginSuccessTracker{
		limiter: limiter,
	}
}

// RecordSuccess resets rate limit counters for successful authentication
func (lst *loginSuccessTracker) RecordSuccess(username, ip string) {
	lst.mu.Lock()
	defer lst.mu.Unlock()
	
	// Reset account-based counter on successful login
	username = strings.ToLower(strings.TrimSpace(username))
	lst.limiter.accountLimiter.mu.Lock()
	delete(lst.limiter.accountLimiter.requests, username)
	lst.limiter.accountLimiter.mu.Unlock()
	
	// Note: We don't reset IP-based counter to prevent rapid-fire attacks
	// even with valid credentials
}
