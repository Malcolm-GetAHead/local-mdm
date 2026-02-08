package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRequestID(t *testing.T) {
	t.Run("returns empty string when not set", func(t *testing.T) {
		ctx := context.Background()
		requestID := GetRequestID(ctx)
		assert.Equal(t, "", requestID)
	})

	t.Run("returns request ID when set", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestIDKey, "my-request-id")
		requestID := GetRequestID(ctx)
		assert.Equal(t, "my-request-id", requestID)
	})

	t.Run("returns empty string for wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestIDKey, 12345)
		requestID := GetRequestID(ctx)
		assert.Equal(t, "", requestID)
	})

	t.Run("handles nil context value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestIDKey, nil)
		requestID := GetRequestID(ctx)
		assert.Equal(t, "", requestID)
	})
}
