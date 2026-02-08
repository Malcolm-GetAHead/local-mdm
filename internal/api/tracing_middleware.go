package api

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
)

// tracingMiddleware wraps the OpenTelemetry mux instrumentation.
// It automatically creates spans for all HTTP requests with route information.
func tracingMiddleware(next http.Handler) http.Handler {
	return otelmux.Middleware("local-mdm")(next)
}
