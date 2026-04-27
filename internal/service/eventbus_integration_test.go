package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDSN() string {
	host := "localhost"
	if h := os.Getenv("DB_HOST"); h != "" {
		host = h
	}
	pass := "postgres-dev-password-1234"
	if p := os.Getenv("DB_PASSWORD"); p != "" {
		pass = p
	}
	return fmt.Sprintf("host=%s port=5432 user=postgres password=%s dbname=localmdm sslmode=disable", host, pass)
}

func TestEventBus_StartStop_RealDB(t *testing.T) {
	eb := NewEventBus(testDSN(), nil, slog.Default())

	err := eb.Start(context.Background())
	if err != nil {
		t.Skipf("skipping: database unavailable: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Shutdown must complete quickly
	done := make(chan struct{})
	go func() {
		eb.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete within 5 seconds")
	}
}

func TestEventBus_StartStop_BadDSN(t *testing.T) {
	eb := NewEventBus("host=192.0.2.1 port=5432 dbname=nope sslmode=disable connect_timeout=1", nil, slog.Default())

	err := eb.Start(context.Background())
	// Should return an error, not hang forever
	assert.Error(t, err, "Start with unreachable host should return error")

	// Shutdown must be safe even after failed Start
	eb.Shutdown()
}

func TestEventBus_StartStop_EmptyDSN(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	err := eb.Start(context.Background())
	assert.Error(t, err, "Start with empty DSN should return error")

	eb.Shutdown()
}

func TestEventBus_ReceivesNotify(t *testing.T) {
	dsn := testDSN()
	eb := NewEventBus(dsn, nil, slog.Default())

	received := make(chan MDMEvent, 1)
	eb.Subscribe("test.event", func(ctx context.Context, event MDMEvent) error {
		received <- event
		return nil
	})

	err := eb.Start(context.Background())
	if err != nil {
		t.Skipf("skipping: database unavailable: %v", err)
	}
	defer eb.Shutdown()

	// Fire a NOTIFY via a separate connection
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`SELECT pg_notify('mdm_events', '{"type":"test.event","id":"00000000-0000-0000-0000-000000000001","device_id":null,"table":"test","op":"INSERT"}')`)
	require.NoError(t, err)

	select {
	case ev := <-received:
		assert.Equal(t, "test.event", ev.Type)
		assert.Equal(t, "INSERT", ev.Op)
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive NOTIFY within 5 seconds")
	}
}
