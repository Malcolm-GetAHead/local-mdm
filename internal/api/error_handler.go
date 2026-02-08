package api

import (
	"log/slog"
	"net/http"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
)

// HandleError processes an error and sends an appropriate HTTP response
// It logs internal error details but only sends sanitized messages to clients
func HandleError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	if err == nil {
		return
	}
	
	// Convert to AppError
	appErr := apperrors.AsAppError(err)
	
	// Log internal error details (never sent to client)
	if appErr.Internal != nil {
		requestID, _ := r.Context().Value(requestIDKey).(string)
		logger.Error("Request failed",
			"request_id", requestID,
			"error", appErr.Internal.Error(),
			"path", r.URL.Path,
			"method", r.Method,
			"code", appErr.Code,
		)
	}
	
	// Return sanitized error to client
	respondError(w, r, appErr.StatusCode, string(appErr.Code), appErr.Message)
}
