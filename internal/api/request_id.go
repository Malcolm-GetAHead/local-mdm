package api

import "context"

// GetRequestID extracts the request ID from the context
// This is a convenience function for handlers and other code that needs the request ID
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}
