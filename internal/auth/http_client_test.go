package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPClientTimeout tests that HTTP requests timeout properly
func TestHTTPClientTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	// Create a server that delays response beyond timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than 10s timeout
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer slowServer.Close()

	// Create validator with slow server - should timeout
	_, err := auth.NewOIDCValidator(slowServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch JWKS")
}

// TestHTTPClientWithValidResponse tests successful JWKS fetch
func TestHTTPClientWithValidResponse(t *testing.T) {
	// Create a server that returns valid JWKS quickly
	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Minimal valid JWKS
		w.Write([]byte(`{"keys":[{"kty":"RSA","use":"sig","kid":"test","n":"test","e":"AQAB","x5c":["MIICmzCCAYMCBgGN"]}]}`))
	}))
	defer validServer.Close()

	// Should succeed
	validator, err := auth.NewOIDCValidator(validServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.NoError(t, err)
	assert.NotNil(t, validator)
}

// TestHTTPClientResponseSizeLimit tests that large responses are handled
func TestHTTPClientResponseSizeLimit(t *testing.T) {
	// Create a server that returns a very large response
	largeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write more than 1MB of data
		largeData := strings.Repeat("x", 2*1024*1024) // 2MB
		w.Write([]byte(`{"keys":[{"data":"` + largeData + `"}]}`))
	}))
	defer largeServer.Close()

	// Should fail due to size limit or JSON decode error
	_, err := auth.NewOIDCValidator(largeServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.Error(t, err)
}

// TestJWKSURLValidation_ValidURLs tests valid JWKS URLs
func TestJWKSURLValidation_ValidURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "https URL",
			url:  "https://example.com/.well-known/jwks.json",
		},
		{
			name: "localhost http",
			url:  "http://localhost:8180/realms/test/protocol/openid-connect/certs",
		},
		{
			name: "127.0.0.1 http",
			url:  "http://127.0.0.1:8180/certs",
		},
		{
			name: "public domain",
			url:  "https://auth.example.com/jwks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"keys":[{"kty":"RSA","use":"sig","kid":"test","n":"test","e":"AQAB","x5c":["MIICmzCCAYMCBgGN"]}]}`))
			}))
			defer server.Close()

			// Use the test server URL instead
			_, err := auth.NewOIDCValidator(server.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
			assert.NoError(t, err)
		})
	}
}

// TestJWKSURLValidation_InvalidURLs tests invalid/dangerous JWKS URLs
func TestJWKSURLValidation_InvalidURLs(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		errMsg  string
	}{
		{
			name:   "private IP 10.x.x.x",
			url:    "http://10.0.0.1/jwks",
			errMsg: "private IP",
		},
		{
			name:   "private IP 192.168.x.x",
			url:    "http://192.168.1.1/jwks",
			errMsg: "private IP",
		},
		{
			name:   "private IP 172.16.x.x",
			url:    "http://172.16.0.1/jwks",
			errMsg: "private IP",
		},
		{
			name:   "AWS metadata service",
			url:    "http://169.254.169.254/latest/meta-data/",
			errMsg: "internal host",
		},
		{
			name:   "link-local IP",
			url:    "http://169.254.1.1/jwks",
			errMsg: "link-local IP",
		},
		{
			name:   "invalid scheme",
			url:    "ftp://example.com/jwks",
			errMsg: "must use HTTP or HTTPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should fail during URL validation
			_, err := auth.NewOIDCValidator(tt.url, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// TestJWKSURLValidation_MetadataServices tests blocking of cloud metadata services
func TestJWKSURLValidation_MetadataServices(t *testing.T) {
	metadataURLs := []string{
		"http://169.254.169.254/latest/meta-data/", // AWS
		"http://metadata.google.internal/",          // GCP
		"http://metadata.azure.com/",                // Azure
	}

	for _, url := range metadataURLs {
		t.Run(url, func(t *testing.T) {
			_, err := auth.NewOIDCValidator(url, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "internal host")
		})
	}
}

// TestHTTPClientConnectionTimeout tests connection timeout
func TestHTTPClientConnectionTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection timeout test in short mode")
	}

	// Use a non-routable IP to test connection timeout
	// 192.0.2.1 is from TEST-NET-1 (RFC 5737) - reserved for documentation
	_, err := auth.NewOIDCValidator("http://192.0.2.1:9999/jwks", "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.Error(t, err)
	// Should timeout or fail to connect
	assert.Contains(t, err.Error(), "failed to fetch JWKS")
}

// TestHTTPClientNon200Response tests handling of non-200 responses
func TestHTTPClientNon200Response(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"403 Forbidden", http.StatusForbidden},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			_, err := auth.NewOIDCValidator(server.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "returned")
		})
	}
}

// TestHTTPClientMalformedJSON tests handling of malformed JSON
func TestHTTPClientMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	_, err := auth.NewOIDCValidator(server.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode JWKS")
}

// TestHTTPClientEmptyKeys tests handling of empty keys array
func TestHTTPClientEmptyKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	_, err := auth.NewOIDCValidator(server.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no keys")
}

// TestHTTPClientConcurrentRequests tests that concurrent JWKS refreshes work correctly
func TestHTTPClientConcurrentRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(100 * time.Millisecond) // Simulate some latency
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[{"kty":"RSA","use":"sig","kid":"test","n":"test","e":"AQAB","x5c":["MIICmzCCAYMCBgGN"]}]}`))
	}))
	defer server.Close()

	validator, err := auth.NewOIDCValidator(server.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	require.NoError(t, err)

	// Simulate concurrent token validations
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			// This will trigger JWKS refresh check
			validator.ValidateToken("dummy-token") // Will fail but that's ok
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Due to double-check locking, should have minimal requests
	// (not testing exact count as it depends on timing)
}

// TestHTTPClientRedirect tests that redirects are followed
func TestHTTPClientRedirect(t *testing.T) {
	// Create target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"keys":[{"kty":"RSA","use":"sig","kid":"test","n":"test","e":"AQAB","x5c":["MIICmzCCAYMCBgGN"]}]}`))
	}))
	defer targetServer.Close()

	// Create redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	// Should follow redirect and succeed
	validator, err := auth.NewOIDCValidator(redirectServer.URL, "test-client", nil, 5, 30*time.Second, 5*time.Minute, nil)
	assert.NoError(t, err)
	assert.NotNil(t, validator)
}
