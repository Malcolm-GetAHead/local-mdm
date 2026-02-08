package tracing

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitTracer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tp, err := InitTracer("test-service", "1.0.0", logger)
	require.NoError(t, err)
	require.NotNil(t, tp)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = Shutdown(ctx, tp)
	assert.NoError(t, err)
}

func TestInitTracer_WithoutLogger(t *testing.T) {
	tp, err := InitTracer("test-service", "1.0.0", nil)
	require.NoError(t, err)
	require.NotNil(t, tp)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = Shutdown(ctx, tp)
	assert.NoError(t, err)
}

func TestShutdown_NilProvider(t *testing.T) {
	ctx := context.Background()
	err := Shutdown(ctx, nil)
	assert.NoError(t, err)
}

func TestShutdown_WithTimeout(t *testing.T) {
	tp, err := InitTracer("test-service", "1.0.0", nil)
	require.NoError(t, err)

	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Should still succeed (or fail gracefully)
	_ = Shutdown(ctx, tp)
}
