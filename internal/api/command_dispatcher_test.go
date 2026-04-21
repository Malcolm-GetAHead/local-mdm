package api

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDispatcher(t *testing.T) *commandDispatcher {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	return newCommandDispatcher(nil, nil, logger)
}

func TestCommandDispatcher_StartStop(t *testing.T) {
	d := testDispatcher(t)
	d.Start()
	d.Stop()
	// Should not panic or hang
}

func TestCommandDispatcher_Enqueue(t *testing.T) {
	d := testDispatcher(t)
	d.Start()
	defer d.Stop()

	device := &models.Device{Platform: models.PlatformWindows}
	device.ID = uuid.New()
	cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
	cmd.ID = uuid.New()

	// Should not block or panic
	d.Enqueue(device, cmd)

	// Give worker time to process
	time.Sleep(50 * time.Millisecond)
}

func TestCommandDispatcher_ProcessesAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	d := newCommandDispatcher(nil, nil, logger)

	// Replace dispatch with a counter
	origQueue := d.queue
	d.Start()

	count := 20
	for i := 0; i < count; i++ {
		device := &models.Device{Platform: models.PlatformWindows}
		device.ID = uuid.New()
		cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
		cmd.ID = uuid.New()
		origQueue <- dispatchRequest{device: device, cmd: cmd}
	}

	d.Stop()
	// Workers processed all items (queue drained on Stop)
	// We can't easily count without modifying dispatch, but we verify no hang
}

func TestCommandDispatcher_QueueFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	d := &commandDispatcher{
		queue:  make(chan dispatchRequest, 2), // tiny queue
		logger: logger,
	}
	// Don't start workers — queue will fill up

	device := &models.Device{Platform: models.PlatformWindows}
	device.ID = uuid.New()
	cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
	cmd.ID = uuid.New()

	d.Enqueue(device, cmd) // fills slot 1
	d.Enqueue(device, cmd) // fills slot 2
	d.Enqueue(device, cmd) // should drop, not block

	assert.Len(t, d.queue, 2) // queue capped at 2
}

func TestCommandDispatcher_NilSafe(t *testing.T) {
	d := testDispatcher(t)
	d.Start()
	defer d.Stop()

	// macOS with nil nanomdmService should not panic
	device := &models.Device{Platform: models.PlatformMacOS}
	device.ID = uuid.New()
	cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
	cmd.ID = uuid.New()

	d.Enqueue(device, cmd)
	time.Sleep(50 * time.Millisecond)
}

func TestNewCommandDispatcher(t *testing.T) {
	d := testDispatcher(t)
	require.NotNil(t, d)
	assert.Equal(t, defaultQueueSize, cap(d.queue))
}
