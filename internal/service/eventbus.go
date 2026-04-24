package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	Type     string     `json:"type"`
	ID       uuid.UUID  `json:"id"`
	DeviceID *uuid.UUID `json:"device_id"`
	Table    string     `json:"table"`
	Op       string     `json:"op"`
}

// EventHandler processes an event. Errors are logged but don't stop the bus.
type EventHandler func(ctx context.Context, event MDMEvent) error

// EventBus listens on PostgreSQL LISTEN/NOTIFY and dispatches to subscribers.
type EventBus struct {
	dsn         string
	listener    *pq.Listener
	subscribers map[string][]EventHandler
	mu          sync.RWMutex
	logger      *slog.Logger
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewEventBus creates an EventBus. Call Start() to begin listening.
func NewEventBus(dsn string, logger *slog.Logger) *EventBus {
	return &EventBus{
		dsn:         dsn,
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

	for {
		select {
		case <-ctx.Done():
			return

		case n := <-eb.listener.Notify:
			if n == nil {
				// nil notification means the connection was lost and re-established.
				// pq.Listener handles reconnection; we just log it.
				eb.logger.Warn("eventbus: received nil notification (reconnect)")
				continue
			}
			eb.dispatch(ctx, n.Extra)

		case <-pingTicker.C:
			if err := eb.listener.Ping(); err != nil {
				eb.logger.Error("eventbus: ping failed", "error", err)
			}
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
		}
	}
}
