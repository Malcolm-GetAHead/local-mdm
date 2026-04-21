package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
)

type mockHook struct {
	unenrollCalled bool
	wipeCalled     bool
	deleteCalled   bool
	lastDevice     *models.Device
	err            error
}

func (m *mockHook) OnUnenroll(_ context.Context, d *models.Device) error {
	m.unenrollCalled = true
	m.lastDevice = d
	return m.err
}
func (m *mockHook) OnWipe(_ context.Context, d *models.Device) error {
	m.wipeCalled = true
	m.lastDevice = d
	return m.err
}
func (m *mockHook) OnDelete(_ context.Context, d *models.Device) error {
	m.deleteCalled = true
	m.lastDevice = d
	return m.err
}

func testDevice() *models.Device {
	return &models.Device{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Platform:  models.PlatformMacOS,
		DeviceID:  "test-udid",
	}
}

func TestLifecycleService_OnUnenroll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)
	hook := &mockHook{}
	svc.RegisterHook(hook)

	device := testDevice()
	svc.OnUnenroll(context.Background(), device)

	assert.True(t, hook.unenrollCalled)
	assert.Equal(t, device.ID, hook.lastDevice.ID)
}

func TestLifecycleService_OnWipe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)
	hook := &mockHook{}
	svc.RegisterHook(hook)

	device := testDevice()
	svc.OnWipe(context.Background(), device)

	assert.True(t, hook.wipeCalled)
}

func TestLifecycleService_OnDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)
	hook := &mockHook{}
	svc.RegisterHook(hook)

	device := testDevice()
	svc.OnDelete(context.Background(), device)

	assert.True(t, hook.deleteCalled)
}

func TestLifecycleService_NoHooks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)

	// Should not panic with no hooks registered
	svc.OnUnenroll(context.Background(), testDevice())
	svc.OnWipe(context.Background(), testDevice())
	svc.OnDelete(context.Background(), testDevice())
}

func TestLifecycleService_HookErrorContinues(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)

	failHook := &mockHook{err: fmt.Errorf("external service down")}
	successHook := &mockHook{}
	svc.RegisterHook(failHook)
	svc.RegisterHook(successHook)

	svc.OnUnenroll(context.Background(), testDevice())

	assert.True(t, failHook.unenrollCalled, "failing hook should still be called")
	assert.True(t, successHook.unenrollCalled, "second hook should be called even if first fails")
}

func TestLifecycleService_MultipleHooks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewLifecycleService(logger)

	hooks := make([]*mockHook, 3)
	for i := range hooks {
		hooks[i] = &mockHook{}
		svc.RegisterHook(hooks[i])
	}

	svc.OnWipe(context.Background(), testDevice())

	for i, h := range hooks {
		assert.True(t, h.wipeCalled, "hook %d should be called", i)
	}
}
