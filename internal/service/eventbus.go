package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	// Channel is the PostgreSQL NOTIFY channel all triggers fire on.
	EventChannel = "mdm_events"

	// MinReconnectInterval is the initial backoff for pq.Listener reconnection.
	MinReconnectInterval = 1 * time.Second

	// MaxReconnectInterval caps the exponential backoff.
	MaxReconnectInterval = 30 * time.Second

	// PingInterval is how often we ping the connection to keep it alive.
	PingInterval = 30 * time.Second
)

// MDMEvent is the JSON payload fired by PostgreSQL triggers.
type MDMEvent struct {
	Type     string                 `json:"type"`
	ID       uuid.UUID              `json:"id"`
	DeviceID *uuid.UUID             `json:"device_id"`
	Table    string                 `json:"table"`
	Op       string                 `json:"op"`
	Extra    map[string]interface{} `json:"extra,omitempty"`
}

// ExtraUUID extracts a UUID from the Extra map.
func (e MDMEvent) ExtraUUID(key string) (uuid.UUID, bool) {
	v, ok := e.Extra[key]
	if !ok {
		return uuid.Nil, false
	}
	s, ok := v.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// ExtraString extracts a string from the Extra map.
func (e MDMEvent) ExtraString(key string) (string, bool) {
	v, ok := e.Extra[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// EventHandler processes an event. Errors are logged but don't stop the bus.
type EventHandler func(ctx context.Context, event MDMEvent) error

// EventBus listens on PostgreSQL LISTEN/NOTIFY and dispatches to subscribers.
type EventBus struct {
	dsn              string
	db               *sql.DB
	listener         *pq.Listener
	subscribers      map[string][]EventHandler
	mu               sync.RWMutex
	logger           *slog.Logger
	cancel           context.CancelFunc
	done             chan struct{}
	retriesExhausted atomic.Int64
}

// NewEventBus creates an EventBus. Call Start() to begin listening.
func NewEventBus(dsn string, db *sql.DB, logger *slog.Logger) *EventBus {
	return &EventBus{
		dsn:         dsn,
		db:          db,
		subscribers: make(map[string][]EventHandler),
		logger:      logger,
		done:        make(chan struct{}),
	}
}

// Subscribe registers a handler for a specific event type.
// Must be called before Start().
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// Start opens the pq.Listener, subscribes to the channel, and begins dispatching.
func (eb *EventBus) Start(ctx context.Context) error {
	// Verify connectivity before creating the listener.
	// pq.NewListener starts a background reconnect goroutine that we can't
	// stop if the initial connection fails, so we pre-flight check here.
	dsn := eb.dsn
	if dsn == "" {
		return fmt.Errorf("eventbus: empty DSN")
	}
	// Append connect_timeout if not already present so we don't hang.
	if !strings.Contains(dsn, "connect_timeout") {
		dsn += " connect_timeout=5"
	}

	probe, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("eventbus: invalid DSN: %w", err)
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer probeCancel()
	if err := probe.PingContext(probeCtx); err != nil {
		probe.Close()
		return fmt.Errorf("eventbus: database unreachable: %w", err)
	}
	probe.Close()

	eb.listener = pq.NewListener(dsn, MinReconnectInterval, MaxReconnectInterval, func(ev pq.ListenerEventType, err error) {
		switch ev {
		case pq.ListenerEventConnected:
			eb.logger.Info("eventbus: connected to PostgreSQL")
		case pq.ListenerEventReconnected:
			eb.logger.Info("eventbus: reconnected to PostgreSQL")
		case pq.ListenerEventDisconnected:
			eb.logger.Error("eventbus: disconnected from PostgreSQL", "error", err)
		case pq.ListenerEventConnectionAttemptFailed:
			eb.logger.Error("eventbus: reconnect attempt failed", "error", err)
		}
	})

	if err := eb.listener.Listen(EventChannel); err != nil {
		eb.listener.Close()
		return fmt.Errorf("eventbus: failed to LISTEN on %s: %w", EventChannel, err)
	}

	listenCtx, cancel := context.WithCancel(ctx)
	eb.cancel = cancel

	go eb.loop(listenCtx)

	eb.logger.Info("eventbus: started", "channel", EventChannel)
	return nil
}

// RetriesExhausted returns the count of events that exhausted all retries since startup.
func (eb *EventBus) RetriesExhausted() int64 {
	return eb.retriesExhausted.Load()
}

// Shutdown stops the listener loop and closes the connection.
func (eb *EventBus) Shutdown() {
	if eb.cancel != nil {
		eb.cancel()
		<-eb.done
	}
	if eb.listener != nil {
		eb.listener.Close()
	}
	eb.logger.Info("eventbus: stopped")
}

// loop is the main dispatch loop. It reads notifications and pings periodically.
func (eb *EventBus) loop(ctx context.Context) {
	defer close(eb.done)

	pingTicker := time.NewTicker(PingInterval)
	defer pingTicker.Stop()

	retryTicker := time.NewTicker(60 * time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case n := <-eb.listener.Notify:
			if n == nil {
				eb.logger.Warn("eventbus: received nil notification (reconnect)")
				continue
			}
			eb.dispatch(ctx, n.Extra)

		case <-pingTicker.C:
			if err := eb.listener.Ping(); err != nil {
				eb.logger.Error("eventbus: ping failed", "error", err)
			}

		case <-retryTicker.C:
			eb.ProcessRetries(ctx)
		}
	}
}

// dispatch parses the JSON payload and calls matching subscribers.
func (eb *EventBus) dispatch(ctx context.Context, payload string) {
	var event MDMEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		eb.logger.Error("eventbus: failed to parse event", "error", err, "payload", payload)
		return
	}

	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		return
	}

	eb.logger.Info("eventbus: dispatching",
		"type", event.Type,
		"id", event.ID,
		"device_id", event.DeviceID,
		"op", event.Op,
	)

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			eb.logger.Error("eventbus: subscriber error",
				"type", event.Type,
				"error", err,
			)
			eb.enqueueRetry(event, err)
		}
	}
}

// enqueueRetry inserts a failed event into event_queue for later retry.
func (eb *EventBus) enqueueRetry(event MDMEvent, handlerErr error) {
	if eb.db == nil {
		return
	}
	payload, _ := json.Marshal(event)
	_, err := eb.db.Exec(
		`INSERT INTO event_queue (event_type, payload, last_error, next_retry_at) VALUES ($1, $2, $3, NOW() + interval '30 seconds')`,
		event.Type, payload, handlerErr.Error(),
	)
	if err != nil {
		eb.logger.Error("eventbus: failed to enqueue retry", "error", err)
	}
}

// ProcessRetries checks event_queue for pending retries and re-dispatches them.
// Call periodically from a background goroutine.
func (eb *EventBus) ProcessRetries(ctx context.Context) {
	if eb.db == nil {
		return
	}
	rows, err := eb.db.QueryContext(ctx,
		`SELECT id, event_type, payload, retry_count FROM event_queue
		 WHERE completed_at IS NULL AND retry_count < max_retries AND next_retry_at <= NOW()
		 ORDER BY next_retry_at LIMIT 10`)
	if err != nil {
		eb.logger.Error("eventbus: retry query failed", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var eventType string
		var payload []byte
		var retryCount int
		if err := rows.Scan(&id, &eventType, &payload, &retryCount); err != nil {
			continue
		}

		var event MDMEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			eb.db.Exec(`UPDATE event_queue SET completed_at = NOW(), last_error = $1 WHERE id = $2`, "invalid payload", id)
			continue
		}

		eb.mu.RLock()
		handlers := eb.subscribers[event.Type]
		eb.mu.RUnlock()

		success := true
		var lastErr string
		for _, h := range handlers {
			if err := h(ctx, event); err != nil {
				success = false
				lastErr = err.Error()
			}
		}

		if success {
			eb.db.Exec(`UPDATE event_queue SET completed_at = NOW() WHERE id = $1`, id)
			eb.logger.Info("eventbus: retry succeeded", "id", id, "type", eventType, "attempt", retryCount+1)
		} else {
			newCount := retryCount + 1
			if newCount >= 5 { // max_retries
				eb.db.Exec(`UPDATE event_queue SET retry_count = $1, last_error = $2, completed_at = NOW() WHERE id = $3`,
					newCount, "exhausted: "+lastErr, id)
				eb.retriesExhausted.Add(1)
				eb.logger.Error("eventbus: retries exhausted", "id", id, "type", eventType, "error", lastErr)
				// Log to audit_logs
				eb.db.Exec(`INSERT INTO audit_logs (enterprise_id, action, resource_type, details) VALUES ('00000000-0000-0000-0000-000000000000', 'eventbus.retry_exhausted', $1, $2)`,
					eventType, fmt.Sprintf(`{"event_id": "%s", "error": "%s", "attempts": %d}`, id, lastErr, newCount))
			} else {
				backoff := time.Duration(1<<uint(newCount)) * 30 * time.Second
				eb.db.Exec(`UPDATE event_queue SET retry_count = $1, last_error = $2, next_retry_at = NOW() + $3::interval WHERE id = $4`,
					newCount, lastErr, fmt.Sprintf("%d seconds", int(backoff.Seconds())), id)
			}
			eb.logger.Warn("eventbus: retry failed", "id", id, "type", eventType, "attempt", newCount, "error", lastErr)
		}
	}
}
