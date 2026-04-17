package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Middleware returns an HTTP middleware that records request metrics.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.HTTPActiveRequests.Inc()
		start := time.Now()

		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		m.HTTPActiveRequests.Dec()

		// Use mux route template if available, otherwise raw path
		path := r.URL.Path
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				path = tmpl
			}
		}

		m.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(wrapped.status)).Inc()
		m.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
