package audit

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

// AsyncLogger writes audit events asynchronously to prevent blocking requests
type AsyncLogger struct {
	logger     *Logger
	eventQueue chan Event
	slogger    *slog.Logger
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
}

// NewAsyncLogger creates a new async audit logger with background workers
func NewAsyncLogger(db *sql.DB, bufferSize, workerCount int, logger *slog.Logger) *AsyncLogger {
	if bufferSize <= 0 {
		bufferSize = 1000 // Default buffer size
	}
	if workerCount <= 0 {
		workerCount = 3 // Default worker count
	}

	al := &AsyncLogger{
		logger:     NewLogger(db),
		eventQueue: make(chan Event, bufferSize),
		slogger:    logger,
	}

	// Set logger for sync logger too
	if logger != nil {
		al.logger.SetLogger(logger)
	}

	// Start background workers
	for i := 0; i < workerCount; i++ {
		al.wg.Add(1)
		go al.worker(i)
	}

	return al
}

// Log queues an audit event for async processing
// Never blocks - drops events if queue is full
func (al *AsyncLogger) Log(ctx context.Context, event Event) error {
	al.mu.Lock()
	if al.closed {
		al.mu.Unlock()
		return nil // Silently ignore after close
	}
	al.mu.Unlock()

	select {
	case al.eventQueue <- event:
		// Queued successfully
		return nil
	default:
		// Queue full - log warning but don't block request
		if al.slogger != nil {
			al.slogger.Warn("Audit log queue full, dropping event",
				"action", event.Action,
				"resource_type", event.ResourceType,
				"enterprise_id", event.EnterpriseID,
			)
		}
		return nil // Don't return error - graceful degradation
	}
}

// worker processes events from the queue
func (al *AsyncLogger) worker(id int) {
	defer al.wg.Done()

	for event := range al.eventQueue {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		if err := al.logger.Log(ctx, event); err != nil {
			if al.slogger != nil {
				al.slogger.Error("Failed to write audit log",
					"error", err,
					"worker_id", id,
					"action", event.Action,
					"resource_type", event.ResourceType,
					"enterprise_id", event.EnterpriseID,
				)
			}
		}

		cancel()
	}
}

// Close gracefully shuts down the async logger
// Waits for all queued events to be processed
func (al *AsyncLogger) Close() error {
	return al.Shutdown(context.Background())
}

// Shutdown gracefully shuts down the async logger with timeout
// Waits for workers to drain queue or context timeout
// If ctx is nil, waits indefinitely for workers to finish
func (al *AsyncLogger) Shutdown(ctx context.Context) error {
	al.mu.Lock()
	if al.closed {
		al.mu.Unlock()
		return nil
	}
	al.closed = true
	al.mu.Unlock()

	close(al.eventQueue)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		al.wg.Wait()
		close(done)
	}()

	// If no context provided, wait indefinitely
	if ctx == nil {
		<-done
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if al.slogger != nil {
			al.slogger.Warn("Audit logger shutdown timeout, some events may be lost")
		}
		return ctx.Err()
	}
}
