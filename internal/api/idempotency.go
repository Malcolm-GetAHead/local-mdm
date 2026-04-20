package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

const (
	idempotencyKeyHeader = "Idempotency-Key"
	idempotencyKeyMaxLen = 255
	idempotencyKeyTTL    = 24 * time.Hour
)

// idempotencyMiddleware returns cached responses for duplicate POST requests
// that carry the same Idempotency-Key header.
func idempotencyMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to POST/PUT/PATCH
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(idempotencyKeyHeader)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			if len(key) > idempotencyKeyMaxLen {
				respondError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key too long")
				return
			}

			// Check for existing response
			var statusCode int
			var headersJSON []byte
			var body []byte
			err := db.QueryRowContext(r.Context(),
				`SELECT status_code, response_headers, response_body
				 FROM idempotency_keys
				 WHERE key = $1 AND method = $2 AND path = $3 AND expires_at > NOW()`,
				key, r.Method, r.URL.Path,
			).Scan(&statusCode, &headersJSON, &body)

			if err == nil {
				// Return cached response
				var headers map[string]string
				if len(headersJSON) > 0 {
					_ = json.Unmarshal(headersJSON, &headers)
				}
				for k, v := range headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(statusCode)
				w.Write(body)
				return
			}

			// Capture the response
			rec := &responseRecorder{ResponseWriter: w, body: &bytes.Buffer{}, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Store the response for future duplicate requests
			respHeaders := map[string]string{"Content-Type": rec.Header().Get("Content-Type")}
			headersData, _ := json.Marshal(respHeaders)

			_, _ = db.ExecContext(r.Context(),
				`INSERT INTO idempotency_keys (key, method, path, status_code, response_headers, response_body, expires_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)
				 ON CONFLICT (key) DO NOTHING`,
				key, r.Method, r.URL.Path, rec.statusCode, headersData, rec.body.Bytes(),
				time.Now().Add(idempotencyKeyTTL),
			)
		})
	}
}

// responseRecorder captures the response for caching while also writing to the client.
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// cleanupIdempotencyKeys removes expired entries. Call periodically.
func cleanupIdempotencyKeys(db *sql.DB) (int64, error) {
	result, err := db.Exec(`DELETE FROM idempotency_keys WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
