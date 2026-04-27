package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sets_common_headers_on_api", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/devices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
		assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	})

	t.Run("dashboard_csp_has_nonce_and_img_src", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboard/devices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		csp := w.Header().Get("Content-Security-Policy")
		assert.Contains(t, csp, "script-src 'self' 'nonce-")
		assert.Contains(t, csp, "style-src 'self' 'nonce-")
		assert.Contains(t, csp, "img-src 'self' data:")
		assert.Contains(t, csp, "connect-src 'self'")
	})

	t.Run("docs_csp_allows_self_and_data_images", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		csp := w.Header().Get("Content-Security-Policy")
		assert.Contains(t, csp, "script-src 'self'")
		assert.Contains(t, csp, "style-src 'self'")
		assert.Contains(t, csp, "img-src 'self' data:")
		// Must NOT contain unsafe-inline or external CDN
		assert.NotContains(t, csp, "unsafe-inline")
		assert.NotContains(t, csp, "unpkg.com")
	})

	t.Run("no_hsts_without_tls", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/devices", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	})

	t.Run("static_gets_default_csp", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/js/app.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		csp := w.Header().Get("Content-Security-Policy")
		// Static paths get the dashboard CSP (nonce-based) since the prefix matches
		assert.Contains(t, csp, "nonce-")
	})
}

func TestSecurityHeadersMiddleware_CSPNonceUniqueness(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var nonces []string
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/dashboard/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		csp := w.Header().Get("Content-Security-Policy")
		// Extract nonce from "script-src 'self' 'nonce-XXXXX'"
		parts := strings.Split(csp, "'nonce-")
		if len(parts) > 1 {
			nonce := strings.Split(parts[1], "'")[0]
			nonces = append(nonces, nonce)
		}
	}

	assert.Len(t, nonces, 5, "should extract 5 nonces")
	// All nonces should be unique
	seen := map[string]bool{}
	for _, n := range nonces {
		assert.False(t, seen[n], "nonce should be unique: %s", n)
		seen[n] = true
	}
}
