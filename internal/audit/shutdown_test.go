package audit

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestEnterprise creates a test enterprise and returns its ID
func createTestEnterprise(t testing.TB, database *db.DB) uuid.UUID {
	t.Helper()

	enterpriseID := uuid.New()
	_, err := database.Writer.Exec(`
		INSERT INTO enterprises (id, name, slug)
		VALUES ($1, $2, $3)
	`, enterpriseID, "Test Enterprise", "test-"+enterpriseID.String())
	require.NoError(t, err)

	return enterpriseID
}

// createTestUser creates a test user and returns its ID
func createTestUser(t testing.TB, database *db.DB, enterpriseID uuid.UUID) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := database.Writer.Exec(`
		INSERT INTO users (id, enterprise_id, email, password_hash, role)
		VALUES ($1, $2, $3, 'hash', 'admin')
	`, userID, enterpriseID, "test-"+userID.String()+"@example.com")
	require.NoError(t, err)

	return userID
}

// TestAsyncLogger_Shutdown_DrainsQueue tests that shutdown waits for queue to drain
func TestAsyncLogger_Shutdown_DrainsQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 100, 3, nil)

	// Create test enterprise and user
	enterpriseID := createTestEnterprise(t, db)
	userID := createTestUser(t, db, enterpriseID)

	// Queue multiple events
	for i := 0; i < 10; i++ {
		err := logger.Log(context.Background(), Event{
			EnterpriseID: enterpriseID,
			UserID:       userID,
			Action:       "test.shutdown",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	}

	// Shutdown with generous timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := logger.Shutdown(ctx)
	require.NoError(t, err)

	// Verify all events were written
	var count int
	err = db.Writer.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'test.shutdown' AND enterprise_id = $1", enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count, "All events should be written before shutdown")
}

// TestAsyncLogger_Shutdown_RespectsTimeout tests that shutdown respects context timeout
func TestAsyncLogger_Shutdown_RespectsTimeout(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create logger with slow worker (simulate stuck worker)
	logger := &AsyncLogger{
		logger:     NewLogger(db.Writer),
		eventQueue: make(chan Event, 100),
		slogger:    slog.Default(),
	}

	// Start worker that processes slowly
	logger.wg.Add(1)
	go func() {
		defer logger.wg.Done()
		for range logger.eventQueue {
			time.Sleep(100 * time.Millisecond) // Slow processing
		}
	}()

	// Queue many events
	for i := 0; i < 50; i++ {
		logger.eventQueue <- Event{
			EnterpriseID: uuid.New(),
			UserID:       uuid.New(),
			Action:       "test.timeout",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		}
	}

	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := logger.Shutdown(ctx)
	elapsed := time.Since(start)

	// Should timeout
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 1*time.Second, "Should timeout quickly")
}

// TestAsyncLogger_Shutdown_Idempotent tests that multiple shutdowns are safe
func TestAsyncLogger_Shutdown_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 10, 1, nil)

	// First shutdown
	ctx := context.Background()
	err := logger.Shutdown(ctx)
	require.NoError(t, err)

	// Second shutdown should be no-op
	err = logger.Shutdown(ctx)
	require.NoError(t, err)

	// Third shutdown should be no-op
	err = logger.Shutdown(ctx)
	require.NoError(t, err)
}

// TestAsyncLogger_Close_CallsShutdown tests that Close() calls Shutdown()
func TestAsyncLogger_Close_CallsShutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 10, 1, nil)

	// Create test enterprise and user
	enterpriseID := createTestEnterprise(t, db)
	userID := createTestUser(t, db, enterpriseID)

	// Queue event
	err := logger.Log(context.Background(), Event{
		EnterpriseID: enterpriseID,
		UserID:       userID,
		Action:       "test.close",
		ResourceType: "test",
		ResourceID:   uuid.New(),
	})
	require.NoError(t, err)

	// Close should drain queue
	err = logger.Close()
	require.NoError(t, err)

	// Verify event was written
	var count int
	err = db.Writer.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'test.close' AND enterprise_id = $1", enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestAsyncLogger_Shutdown_EmptyQueue tests shutdown with empty queue
func TestAsyncLogger_Shutdown_EmptyQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 10, 3, nil)

	// Shutdown immediately (no events queued)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := logger.Shutdown(ctx)
	require.NoError(t, err)
}

// TestAsyncLogger_Shutdown_LargeQueue tests shutdown with many events
func TestAsyncLogger_Shutdown_LargeQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 1000, 5, nil) // More workers for speed

	// Create test enterprise
	enterpriseID := createTestEnterprise(t, db)

	// Queue many events (they will fail FK constraint, but that's OK for shutdown test)
	eventCount := 500
	for i := 0; i < eventCount; i++ {
		err := logger.Log(context.Background(), Event{
			EnterpriseID: enterpriseID,
			UserID:       uuid.New(),
			Action:       "test.large",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	}

	// Shutdown with reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := logger.Shutdown(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	t.Logf("Drained %d events in %v (workers processed all events, some may have failed FK constraints)", eventCount, elapsed)

	// Verify shutdown completed (don't check count since FK constraints will cause failures)
	// The important thing is that shutdown waited for workers to finish
}

// TestAsyncLogger_Shutdown_RejectsNewEvents tests that events are rejected after shutdown
func TestAsyncLogger_Shutdown_RejectsNewEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 10, 1, nil)

	// Shutdown
	ctx := context.Background()
	err := logger.Shutdown(ctx)
	require.NoError(t, err)

	// Try to log after shutdown (should be silently ignored)
	err = logger.Log(context.Background(), Event{
		EnterpriseID: uuid.New(),
		UserID:       uuid.New(),
		Action:       "test.after.shutdown",
		ResourceType: "test",
		ResourceID:   uuid.New(),
	})
	require.NoError(t, err) // No error, but event not logged

	// Verify event was NOT written
	var count int
	err = db.Writer.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'test.after.shutdown'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Events after shutdown should be ignored")
}

// TestAsyncLogger_Shutdown_ConcurrentShutdowns tests concurrent shutdown calls
func TestAsyncLogger_Shutdown_ConcurrentShutdowns(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 100, 3, nil)

	// Create test enterprise
	enterpriseID := createTestEnterprise(t, db)

	// Queue some events
	for i := 0; i < 10; i++ {
		err := logger.Log(context.Background(), Event{
			EnterpriseID: enterpriseID,
			UserID:       uuid.New(),
			Action:       "test.concurrent",
			ResourceType: "test",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	}

	// Call shutdown concurrently from multiple goroutines
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done <- logger.Shutdown(ctx)
		}()
	}

	// All should succeed (or be no-op)
	for i := 0; i < 5; i++ {
		err := <-done
		assert.NoError(t, err)
	}
}

// BenchmarkAsyncLogger_Shutdown benchmarks shutdown performance
func BenchmarkAsyncLogger_Shutdown(b *testing.B) {
	db := setupTestDB(b)
	defer db.Close()

	b.Run("empty_queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger := NewAsyncLogger(db.Writer, 100, 3, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = logger.Shutdown(ctx)
			cancel()
		}
	})

	b.Run("small_queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger := NewAsyncLogger(db.Writer, 100, 3, nil)

			// Queue 10 events
			for j := 0; j < 10; j++ {
				_ = logger.Log(context.Background(), Event{
					EnterpriseID: uuid.New(),
					UserID:       uuid.New(),
					Action:       "bench.shutdown",
					ResourceType: "test",
					ResourceID:   uuid.New(),
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = logger.Shutdown(ctx)
			cancel()
		}
	})

	b.Run("large_queue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger := NewAsyncLogger(db.Writer, 1000, 5, nil)

			// Queue 100 events
			for j := 0; j < 100; j++ {
				_ = logger.Log(context.Background(), Event{
					EnterpriseID: uuid.New(),
					UserID:       uuid.New(),
					Action:       "bench.shutdown",
					ResourceType: "test",
					ResourceID:   uuid.New(),
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = logger.Shutdown(ctx)
			cancel()
		}
	})
}

func TestAsyncLogger_Shutdown_NilContext(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger := NewAsyncLogger(db.Writer, 100, 2, nil)

	// Create test enterprise and user
	enterpriseID := createTestEnterprise(t, db)
	userID := createTestUser(t, db, enterpriseID)

	// Log an event
	err := logger.Log(context.Background(), Event{
		EnterpriseID: enterpriseID,
		UserID:       userID,
		Action:       "test.action",
		ResourceType: "test",
		ResourceID:   uuid.New(),
	})
	require.NoError(t, err)

	// Shutdown with nil context should not panic
	err = logger.Shutdown(nil)
	assert.NoError(t, err)

	// Verify event was logged
	var count int
	err = db.Writer.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'test.action' AND user_id = $1", userID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
