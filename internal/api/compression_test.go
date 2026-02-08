package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionMiddleware(t *testing.T) {
	// Create test handler that returns large payload
	largePayload := strings.Repeat("test data ", 1000) // 10KB
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largePayload))
	})

	t.Run("compresses when client accepts gzip", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		compressionMiddleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

		// Decompress and verify
		gz, err := gzip.NewReader(w.Body)
		require.NoError(t, err)
		defer gz.Close()

		decompressed, err := io.ReadAll(gz)
		require.NoError(t, err)
		assert.Equal(t, largePayload, string(decompressed))

		// Verify compression actually reduced size
		assert.Less(t, w.Body.Len(), len(largePayload))
	})

	t.Run("does not compress when client does not accept gzip", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		compressionMiddleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Content-Encoding"))
		assert.Equal(t, largePayload, w.Body.String())
	})

	t.Run("compression ratio for JSON payload", func(t *testing.T) {
		jsonPayload := `{"devices":[` + strings.Repeat(`{"id":"123","name":"device","status":"active"},`, 100) + `]}`
		jsonHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(jsonPayload))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		compressionMiddleware(jsonHandler).ServeHTTP(w, req)

		compressionRatio := float64(w.Body.Len()) / float64(len(jsonPayload))
		assert.Less(t, compressionRatio, 0.5, "JSON should compress to less than 50% of original size")
	})

	t.Run("handles empty response", func(t *testing.T) {
		emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		compressionMiddleware(emptyHandler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("preserves status codes", func(t *testing.T) {
		statusCodes := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusInternalServerError,
		}

		for _, statusCode := range statusCodes {
			statusHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte("test"))
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()

			compressionMiddleware(statusHandler).ServeHTTP(w, req)

			assert.Equal(t, statusCode, w.Code, "status code %d should be preserved", statusCode)
		}
	})

	t.Run("removes content-length header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		compressionMiddleware(handler).ServeHTTP(w, req)

		assert.Empty(t, w.Header().Get("Content-Length"), "Content-Length should be removed after compression")
	})
}

func BenchmarkCompressionMiddleware(b *testing.B) {
	largePayload := strings.Repeat("test data ", 1000)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(largePayload))
	})

	middleware := compressionMiddleware(handler)

	b.Run("with compression", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
		}
	})

	b.Run("without compression", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)
		}
	})
}
