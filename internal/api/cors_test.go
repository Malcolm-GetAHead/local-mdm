package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/config"
)

func TestCORSMiddleware(t *testing.T) {
	t.Run("allows_whitelisted_origin", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Error("Expected origin to be allowed")
		}
	})

	t.Run("blocks_non_whitelisted_origin", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://evil.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("Expected origin to be blocked")
		}
	})

	t.Run("handles_preflight_request", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called for OPTIONS")
		}))

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("sets_credentials_header", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins:   []string{"http://localhost:3000"},
			AllowCredentials: true,
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Error("Expected credentials header to be set")
		}
	})

	t.Run("sets_max_age", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			MaxAge:         3600,
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Max-Age") != "3600" {
			t.Error("Expected max age header to be set")
		}
	})

	t.Run("no_origin_header", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("Expected no CORS headers when no origin")
		}
	})

	t.Run("sets_vary_header", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Vary") != "Origin" {
			t.Error("Expected Vary: Origin header for caching")
		}
	})
}

func TestIsAllowedOrigin(t *testing.T) {
	t.Run("exact_match", func(t *testing.T) {
		allowed := []string{"http://localhost:3000", "https://example.com"}
		
		if !isAllowedOrigin("http://localhost:3000", allowed) {
			t.Error("Expected exact match to be allowed")
		}
		
		if isAllowedOrigin("http://evil.com", allowed) {
			t.Error("Expected non-match to be blocked")
		}
	})

	t.Run("wildcard_all", func(t *testing.T) {
		allowed := []string{"*"}
		
		if !isAllowedOrigin("http://anything.com", allowed) {
			t.Error("Expected wildcard to allow any origin")
		}
	})

	t.Run("wildcard_subdomain", func(t *testing.T) {
		allowed := []string{"*.example.com"}
		
		if !isAllowedOrigin("http://app.example.com", allowed) {
			t.Error("Expected subdomain to be allowed")
		}
		
		if !isAllowedOrigin("https://api.example.com", allowed) {
			t.Error("Expected subdomain to be allowed")
		}
		
		if isAllowedOrigin("http://example.com", allowed) {
			t.Error("Expected root domain to be blocked")
		}
		
		if isAllowedOrigin("http://evil.com", allowed) {
			t.Error("Expected different domain to be blocked")
		}
	})

	t.Run("empty_list", func(t *testing.T) {
		allowed := []string{}
		
		if isAllowedOrigin("http://localhost:3000", allowed) {
			t.Error("Expected empty list to block all")
		}
	})
}

func TestJoinStrings(t *testing.T) {
	result := joinStrings([]string{"GET", "POST", "PUT"})
	expected := "GET, POST, PUT"
	
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCORSIntegration(t *testing.T) {
	t.Run("cors_applied_in_middleware_stack", func(t *testing.T) {
		// Create a test handler that checks if CORS headers are set
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if CORS was applied
			if r.Header.Get("Origin") != "" {
				origin := w.Header().Get("Access-Control-Allow-Origin")
				if origin == "" {
					t.Error("CORS middleware not applied - no Access-Control-Allow-Origin header")
				}
			}
			w.WriteHeader(http.StatusOK)
		})

		cfg := config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "POST"},
		}

		// Apply CORS middleware
		handler := corsMiddleware(cfg)(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("multiple_origins_in_config", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{
				"http://localhost:3000",
				"http://localhost:8080",
				"https://example.com",
			},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test each origin
		origins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"https://example.com",
		}

		for _, origin := range origins {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Errorf("Expected origin %s to be allowed", origin)
			}
		}
	})

	t.Run("empty_config_blocks_all", func(t *testing.T) {
		cfg := config.CORSConfig{
			AllowedOrigins: []string{},
		}

		handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("Expected empty config to block all origins")
		}
	})
}
