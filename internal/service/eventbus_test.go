package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBus_Subscribe(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error {
		return nil
	})

	assert.Len(t, eb.subscribers["device.enrolled"], 1)
	assert.Empty(t, eb.subscribers["policy.updated"])
}

func TestEventBus_SubscribeMultiple(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error { return nil })
	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error { return nil })
	eb.Subscribe("policy.updated", func(ctx context.Context, event MDMEvent) error { return nil })

	assert.Len(t, eb.subscribers["device.enrolled"], 2)
	assert.Len(t, eb.subscribers["policy.updated"], 1)
}

func TestEventBus_Dispatch(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	deviceID := uuid.New()
	var received MDMEvent
	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error {
		received = event
		return nil
	})

	payload, _ := json.Marshal(MDMEvent{
		Type:     "device.enrolled",
		ID:       deviceID,
		DeviceID: &deviceID,
		Table:    "devices",
		Op:       "INSERT",
	})

	eb.dispatch(context.Background(), string(payload))

	assert.Equal(t, "device.enrolled", received.Type)
	assert.Equal(t, deviceID, received.ID)
	assert.Equal(t, &deviceID, received.DeviceID)
	assert.Equal(t, "INSERT", received.Op)
}

func TestEventBus_DispatchNoSubscribers(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	// Should not panic when no subscribers
	payload, _ := json.Marshal(MDMEvent{
		Type:  "unknown.event",
		ID:    uuid.New(),
		Table: "devices",
		Op:    "INSERT",
	})
	eb.dispatch(context.Background(), string(payload))
}

func TestEventBus_DispatchInvalidJSON(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	// Should not panic on bad JSON
	eb.dispatch(context.Background(), "not json")
}

func TestEventBus_DispatchSubscriberError(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	// First handler errors, second should still be called
	var secondCalled bool
	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error {
		return errors.New("handler failed")
	})
	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error {
		secondCalled = true
		return nil
	})

	payload, _ := json.Marshal(MDMEvent{
		Type:  "device.enrolled",
		ID:    uuid.New(),
		Table: "devices",
		Op:    "INSERT",
	})
	eb.dispatch(context.Background(), string(payload))

	assert.True(t, secondCalled, "second handler should be called even if first errors")
}

func TestEventBus_DispatchConcurrentSafe(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	var mu sync.Mutex
	count := 0
	eb.Subscribe("device.enrolled", func(ctx context.Context, event MDMEvent) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	payload, _ := json.Marshal(MDMEvent{
		Type:  "device.enrolled",
		ID:    uuid.New(),
		Table: "devices",
		Op:    "INSERT",
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eb.dispatch(context.Background(), string(payload))
		}()
	}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, 100, count)
	mu.Unlock()
}

func TestEventBus_NullDeviceID(t *testing.T) {
	eb := NewEventBus("", nil, slog.Default())

	var received MDMEvent
	eb.Subscribe("policy.updated", func(ctx context.Context, event MDMEvent) error {
		received = event
		return nil
	})

	// Policy events have null device_id
	payload := `{"type":"policy.updated","id":"` + uuid.New().String() + `","device_id":null,"table":"policies","op":"UPDATE"}`
	eb.dispatch(context.Background(), payload)

	assert.Equal(t, "policy.updated", received.Type)
	assert.Nil(t, received.DeviceID)
}

func TestMDMEvent_JSONRoundTrip(t *testing.T) {
	deviceID := uuid.New()
	original := MDMEvent{
		Type:     "device.enrolled",
		ID:       uuid.New(),
		DeviceID: &deviceID,
		Table:    "devices",
		Op:       "INSERT",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded MDMEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.DeviceID, decoded.DeviceID)
	assert.Equal(t, original.Table, decoded.Table)
	assert.Equal(t, original.Op, decoded.Op)
}
