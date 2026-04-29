package audit

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncLogger_LogsEventsAsynchronously(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	logger := NewAsyncLogger(db.Writer, 100, 3, slog.Default())
	defer logger.Close()

	// Log 10 events
	for i := 0; i < 10; i++ {
		err := logger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			Action:       "test.action",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	}

	// Close and wait for processing
	err := logger.Close()
	require.NoError(t, err)

	// Verify all events were written
	var count int
	err = db.Writer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs 
		WHERE enterprise_id = $1 AND action = 'test.action'
	`, enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count)
}

func TestAsyncLogger_HandlesQueueFull(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	// Capture log output
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Small buffer to force queue full
	logger := NewAsyncLogger(db.Writer, 5, 3, testLogger)
	defer logger.Close()

	// Fill queue beyond capacity
	for i := 0; i < 20; i++ {
		err := logger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			Action:       "test.overflow",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err) // Should never error
	}

	// Verify warning was logged
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "Audit log queue full")
	assert.Contains(t, logOutput, "dropping event")
}

func TestAsyncLogger_MultipleWorkersProcessConcurrently(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	logger := NewAsyncLogger(db.Writer, 1000, 3, slog.Default())
	defer logger.Close()

	// Log 100 events concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := logger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				Action:       "concurrent.test",
				ResourceType: "test",
				ResourceID:   uuid.New(),
			})
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Close and wait for processing
	err := logger.Close()
	require.NoError(t, err)

	// Verify all events were written
	var count int
	err = db.Writer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs 
		WHERE enterprise_id = $1 AND action = 'concurrent.test'
	`, enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 100, count)
}

func TestAsyncLogger_GracefulShutdownDrainsQueue(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	logger := NewAsyncLogger(db.Writer, 100, 3, slog.Default())

	// Queue events
	for i := 0; i < 50; i++ {
		err := logger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			Action:       "shutdown.test",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	}

	// Close immediately (should wait for queue to drain)
	start := time.Now()
	err := logger.Close()
	duration := time.Since(start)
	require.NoError(t, err)

	t.Logf("Shutdown took %v to drain queue", duration)

	// Verify all events were written
	var count int
	err = db.Writer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs 
		WHERE enterprise_id = $1 AND action = 'shutdown.test'
	`, enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 50, count)
}

func TestAsyncLogger_DatabaseFailureDoesNotBlockRequests(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	// Capture error logs
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	logger := NewAsyncLogger(db.Writer, 100, 3, testLogger)
	defer logger.Close()

	// Use invalid enterprise ID to trigger database error
	invalidID := uuid.New()

	// Log should not block even though it will fail
	start := time.Now()
	err := logger.Log(ctx, Event{
		EnterpriseID: invalidID,
		Action:       "failure.test",
		ResourceType: "test",
		ResourceID:   uuid.New(),
	})
	duration := time.Since(start)

	// Should return immediately (not block)
	assert.NoError(t, err)
	assert.Less(t, duration, 100*time.Millisecond, "Log should not block")

	// Wait a bit for worker to process
	time.Sleep(100 * time.Millisecond)

	// Close to ensure worker processed the event
	logger.Close()

	// Verify error was logged
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "Failed to write audit log")
}

func TestAsyncLogger_WorkerErrorsAreLogged(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	// Capture error logs
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	logger := NewAsyncLogger(db.Writer, 100, 3, testLogger)
	defer logger.Close()

	// Log event with invalid data
	err := logger.Log(ctx, Event{
		EnterpriseID: uuid.New(), // Invalid - doesn't exist
		Action:       "error.test",
		ResourceType: "test",
	})
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	logger.Close()

	// Verify error was logged with context
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "Failed to write audit log")
	assert.Contains(t, logOutput, "worker_id")
	assert.Contains(t, logOutput, "error.test")
}

func TestAsyncLogger_ConcurrentWritesAreSafe(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	logger := NewAsyncLogger(db.Writer, 1000, 3, slog.Default())
	defer logger.Close()

	// Concurrent writes from multiple goroutines
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				err := logger.Log(ctx, Event{
					EnterpriseID: enterpriseID,
					Action:       "concurrent.write",
					ResourceType: "test",
					ResourceID:   uuid.New(),
				})
				if err == nil {
					successCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()
	logger.Close()

	// All writes should succeed
	assert.Equal(t, int32(500), successCount.Load())

	// Verify count in database
	var count int
	err := db.Writer.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs 
		WHERE enterprise_id = $1 AND action = 'concurrent.write'
	`, enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 500, count)
}

func TestAsyncLogger_IgnoresEventsAfterClose(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	logger := NewAsyncLogger(db.Writer, 100, 3, slog.Default())

	// Close immediately
	err := logger.Close()
	require.NoError(t, err)

	// Try to log after close (should not panic)
	err = logger.Log(ctx, Event{
		Action:       "after.close",
		ResourceType: "test",
	})
	assert.NoError(t, err) // Should silently ignore
}

func BenchmarkAsyncLogger_vs_SyncLogger(b *testing.B) {
	db := testutil.ConnectDB(b)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(b, db.Writer, "Bench Enterprise")

	b.Run("sync_logger", func(b *testing.B) {
		logger := NewLogger(db.Writer)
		logger.SetLogger(nil) // Disable logging overhead

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				Action:       "bench.sync",
				ResourceType: "test",
				ResourceID:   uuid.New(),
			})
		}
	})

	b.Run("async_logger", func(b *testing.B) {
		logger := NewAsyncLogger(db.Writer, 10000, 3, nil)
		defer logger.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				Action:       "bench.async",
				ResourceType: "test",
				ResourceID:   uuid.New(),
			})
		}
	})
}

func BenchmarkAsyncLogger_HighThroughput(b *testing.B) {
	db := testutil.ConnectDB(b)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(b, db.Writer, "Bench Enterprise")

	logger := NewAsyncLogger(db.Writer, 10000, 3, nil)
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = logger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				Action:       "bench.throughput",
				ResourceType: "test",
				ResourceID:   uuid.New(),
			})
		}
	})
}
