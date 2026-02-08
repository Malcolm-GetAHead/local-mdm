package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/constants"
	"github.com/stretchr/testify/assert"
)

func TestRequestSizeLimitMiddleware(t *testing.T) {
	// Test handler that reads the body
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("received %d bytes", len(body))))
	})

	middleware := requestSizeLimitMiddleware(constants.MaxRequestBodySize)(handler)

	t.Run("accepts_request_within_limit", func(t *testing.T) {
		// Create request with 1KB body (well under 1MB limit)
		body := bytes.Repeat([]byte("a"), 1024)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "received 1024 bytes")
	})

	t.Run("accepts_request_at_exact_limit", func(t *testing.T) {
		// Create request with exactly 1MB body
		body := bytes.Repeat([]byte("a"), constants.MaxRequestBodySize)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("rejects_request_over_limit", func(t *testing.T) {
		// Create request with 1MB + 1 byte
		body := bytes.Repeat([]byte("a"), constants.MaxRequestBodySize+1)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("rejects_large_request", func(t *testing.T) {
		// Create request with 10MB body (10x the limit)
		body := bytes.Repeat([]byte("a"), 10*constants.MaxRequestBodySize)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("handles_empty_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("handles_get_request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("handles_chunked_encoding_within_limit", func(t *testing.T) {
		// Simulate chunked transfer encoding
		body := bytes.Repeat([]byte("a"), 1024)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("Transfer-Encoding", "chunked")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("rejects_chunked_encoding_over_limit", func(t *testing.T) {
		// Simulate chunked transfer encoding with large body
		body := bytes.Repeat([]byte("a"), constants.MaxRequestBodySize+1)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		req.Header.Set("Transfer-Encoding", "chunked")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

func TestRequestSizeLimitMiddleware_JSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	middleware := requestSizeLimitMiddleware(constants.MaxRequestBodySize)(handler)

	t.Run("accepts_valid_json_within_limit", func(t *testing.T) {
		json := `{"name":"test","data":"` + strings.Repeat("x", 1000) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, json, rec.Body.String())
	})

	t.Run("rejects_large_json", func(t *testing.T) {
		// Create JSON larger than 1MB
		largeData := strings.Repeat("x", constants.MaxRequestBodySize)
		json := `{"data":"` + largeData + `"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(json))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

func TestRequestSizeLimitMiddleware_Multipart(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(constants.MaxRequestBodySize)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("multipart parsed"))
	})

	middleware := requestSizeLimitMiddleware(constants.MaxRequestBodySize)(handler)

	t.Run("accepts_multipart_within_limit", func(t *testing.T) {
		body := &bytes.Buffer{}
		body.WriteString("--boundary\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"field\"\r\n\r\n")
		body.WriteString(strings.Repeat("a", 1024))
		body.WriteString("\r\n--boundary--\r\n")

		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("rejects_large_multipart", func(t *testing.T) {
		body := &bytes.Buffer{}
		body.WriteString("--boundary\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"field\"\r\n\r\n")
		body.WriteString(strings.Repeat("a", constants.MaxRequestBodySize+1))
		body.WriteString("\r\n--boundary--\r\n")

		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

func TestRequestSizeLimitMiddleware_EdgeCases(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := requestSizeLimitMiddleware(constants.MaxRequestBodySize)(handler)

	t.Run("handles_nil_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("preserves_request_method", func(t *testing.T) {
		methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		
		for _, method := range methods {
			req := httptest.NewRequest(method, "/test", strings.NewReader("test"))
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			assert.Equal(t, method, req.Method)
		}
	})

	t.Run("preserves_request_headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test"))
		req.Header.Set("X-Custom-Header", "test-value")
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		assert.Equal(t, "test-value", req.Header.Get("X-Custom-Header"))
		assert.Equal(t, "Bearer token", req.Header.Get("Authorization"))
	})
}

func BenchmarkRequestSizeLimitMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	middleware := requestSizeLimitMiddleware(constants.MaxRequestBodySize)(handler)

	b.Run("small_body_1KB", func(b *testing.B) {
		body := bytes.Repeat([]byte("a"), 1024)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)
		}
	})

	b.Run("medium_body_100KB", func(b *testing.B) {
		body := bytes.Repeat([]byte("a"), 100*1024)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)
		}
	})

	b.Run("large_body_1MB", func(b *testing.B) {
		body := bytes.Repeat([]byte("a"), constants.MaxRequestBodySize)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)
		}
	})
}
