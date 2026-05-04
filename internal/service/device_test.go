package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDeviceRepo struct {
	devices map[uuid.UUID]*models.Device
}

func newMockDeviceRepo() *mockDeviceRepo { return &mockDeviceRepo{devices: map[uuid.UUID]*models.Device{}} }

func (m *mockDeviceRepo) Create(_ context.Context, d *models.Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	m.devices[d.ID] = d
	return nil
}
func (m *mockDeviceRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	if d, ok := m.devices[id]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("device not found")
}
func (m *mockDeviceRepo) GetBySerial(_ context.Context, _ uuid.UUID, _ string) (*models.Device, error) {
	return nil, fmt.Errorf("device not found")
}
func (m *mockDeviceRepo) GetByPlatformID(_ context.Context, platform, deviceID string) (*models.Device, error) {
	for _, d := range m.devices {
		if d.Platform == platform && d.DeviceID == deviceID {
			return d, nil
		}
	}
	return nil, fmt.Errorf("device not found")
}
func (m *mockDeviceRepo) GetByPlatformIDIncludeDeleted(_ context.Context, platform, deviceID string) (*models.Device, error) {
	for _, d := range m.devices {
		if d.Platform == platform && d.DeviceID == deviceID {
			return d, nil
		}
	}
	return nil, fmt.Errorf("device not found")
}
func (m *mockDeviceRepo) List(_ context.Context, eid uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	var out []*models.Device
	for _, d := range m.devices {
		if d.EnterpriseID == eid {
			out = append(out, d)
		}
	}
	return out, len(out), nil
}
func (m *mockDeviceRepo) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockDeviceRepo) Update(_ context.Context, d *models.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.devices[id]; !ok {
		return fmt.Errorf("device not found")
	}
	delete(m.devices, id)
	return nil
}

type mockCmdRepo struct {
	commands []*models.DeviceCommand
}

func (m *mockCmdRepo) Create(_ context.Context, cmd *models.DeviceCommand) error {
	cmd.ID = uuid.New()
	m.commands = append(m.commands, cmd)
	return nil
}

type mockDispatcher struct {
	enqueued []*models.DeviceCommand
}

func (m *mockDispatcher) Enqueue(_ *models.Device, cmd *models.DeviceCommand) {
	m.enqueued = append(m.enqueued, cmd)
}

func newTestDevice(platform string) *models.Device {
	id := uuid.New()
	return &models.Device{
		BaseModel:    models.BaseModel{ID: id},
		EnterpriseID: uuid.New(),
		Platform:     platform,
		DeviceID:     id.String(),
		Status:       models.DeviceStatusEnrolled,
	}
}

func TestDeviceService_GetAndList(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	got, err := svc.Get(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got.ID)

	list, total, err := svc.List(context.Background(), d.EnterpriseID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)
}

func TestDeviceService_Update(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformWindows)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	name := "Updated"
	updated, err := svc.Update(context.Background(), d.ID, &name, nil, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
}

func TestDeviceService_Delete(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	hook := &mockHook{}
	ls := NewLifecycleService(testLogger())
	ls.RegisterHook(hook)
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, ls, testLogger())

	err := svc.Delete(context.Background(), d.ID)
	require.NoError(t, err)
	assert.True(t, hook.deleteCalled)
	assert.Empty(t, dr.devices)
}

func TestDeviceService_Lock(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	cr := &mockCmdRepo{}
	disp := &mockDispatcher{}
	svc := NewDeviceService(dr, cr, disp, NewLifecycleService(testLogger()), testLogger())

	result, err := svc.Lock(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceStatusLost, result.Device.Status)
	assert.Equal(t, models.CommandTypeLock, result.Command.CommandType)
	assert.Len(t, disp.enqueued, 1)
}

func TestDeviceService_Wipe(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	hook := &mockHook{}
	ls := NewLifecycleService(testLogger())
	ls.RegisterHook(hook)
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, ls, testLogger())

	result, err := svc.Wipe(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceStatusWiped, result.Device.Status)
	assert.True(t, hook.wipeCalled)
}

func TestDeviceService_Restart(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	result, err := svc.Restart(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CommandTypeRestart, result.Command.CommandType)
}

func TestDeviceService_Restart_WindowsRejected(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformWindows)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	_, err := svc.Restart(context.Background(), d.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestDeviceService_Unenroll(t *testing.T) {
	dr := newMockDeviceRepo()
	d := newTestDevice(models.PlatformMacOS)
	dr.devices[d.ID] = d
	svc := NewDeviceService(dr, &mockCmdRepo{}, &mockDispatcher{}, NewLifecycleService(testLogger()), testLogger())

	result, err := svc.Unenroll(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, "unenrolled", result.Status)
}
