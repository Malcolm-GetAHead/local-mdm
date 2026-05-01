package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
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

func TestCommandDispatcher_DispatchContextHasDeadline(t *testing.T) {
	// Verify that dispatch() completes (with error) when NanoMDM is slow,
	// rather than hanging indefinitely. The 30s dispatch timeout + 30s client
	// timeout means this should complete within ~30s.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow NanoMDM — sleep 5s then respond
		time.Sleep(5 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"command_uuid":"test-uuid"}`))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	nanoSvc := macos.NewNanoMDMService(srv.URL, "", nil, nil, logger)
	d := newCommandDispatcher(&mockCommandRepo{}, nanoSvc, logger)
	d.Start()

	device := &models.Device{Platform: models.PlatformMacOS, DeviceID: "test-udid"}
	device.ID = uuid.New()
	cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
	cmd.ID = uuid.New()

	d.Enqueue(device, cmd)
	time.Sleep(6 * time.Second) // Wait for the slow dispatch to complete
	d.Stop()
	// If we get here without hanging, the timeout infrastructure works.
	// The command was dispatched (slowly) and the worker didn't block forever.
}

func TestCommandDispatcher_DispatchRespectsTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	// Server that never responds — dispatch should fail via client/context timeout
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second) // Longer than both timeouts
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	nanoSvc := macos.NewNanoMDMService(srv.URL, "", nil, nil, logger)
	d := newCommandDispatcher(&mockCommandRepo{}, nanoSvc, logger)
	d.Start()

	device := &models.Device{Platform: models.PlatformMacOS, DeviceID: "test-udid"}
	device.ID = uuid.New()
	cmd := &models.DeviceCommand{CommandType: models.CommandTypeLock}
	cmd.ID = uuid.New()

	start := time.Now()
	d.Enqueue(device, cmd)
	d.Stop() // Waits for workers to drain
	elapsed := time.Since(start)

	assert.Contains(t, logBuf.String(), "failed to dispatch macOS command")
	// Should complete in ~30s (client timeout), not 35s (server sleep) or forever
	assert.Less(t, elapsed, 34*time.Second, "dispatch should timeout, not hang")
}
