package service

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cmdTestCertRepo struct {
	certs []*models.Certificate
}

func (m *cmdTestCertRepo) List(_ context.Context, _ *uuid.UUID, _, _ int) ([]*models.Certificate, int, error) {
	return m.certs, len(m.certs), nil
}

type cmdTestCmdRepo struct {
	commands []*models.DeviceCommand
}

func (m *cmdTestCmdRepo) Create(_ context.Context, cmd *models.DeviceCommand) error {
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	m.commands = append(m.commands, cmd)
	return nil
}
func (m *cmdTestCmdRepo) ListByDevice(_ context.Context, deviceID uuid.UUID, _, _ int) ([]*models.DeviceCommand, int, error) {
	var result []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID {
			result = append(result, c)
		}
	}
	return result, len(result), nil
}

type cmdTestDeviceRepo struct {
	devices []*models.Device
}

func (m *cmdTestDeviceRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Device, error) {
	for _, d := range m.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, fmt.Errorf("not found: %w", apperrors.ErrNotFound)
}
func (m *cmdTestDeviceRepo) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Device, int, error) {
	return m.devices, len(m.devices), nil
}
func (m *cmdTestDeviceRepo) ListFiltered(_ context.Context, _ uuid.UUID, _, _, _, _, _ string, _, _ int) ([]*models.Device, int, error) {
	return m.devices, len(m.devices), nil
}
func (m *cmdTestDeviceRepo) Update(_ context.Context, _ *models.Device) error { return nil }
func (m *cmdTestDeviceRepo) Delete(_ context.Context, _ uuid.UUID) error      { return nil }

func TestCommandService_CreateAndList(t *testing.T) {
	deviceID := uuid.New()
	cmdRepo := &cmdTestCmdRepo{}
	deviceRepo := &cmdTestDeviceRepo{devices: []*models.Device{{BaseModel: models.BaseModel{ID: deviceID}, Platform: "windows"}}}
	certRepo := &cmdTestCertRepo{}
	svc := NewCommandService(cmdRepo, deviceRepo, certRepo, slog.Default())
	ctx := context.Background()

	// GetDevice
	dev, err := svc.GetDevice(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, deviceID, dev.ID)

	// GetDevice not found
	_, err = svc.GetDevice(ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)

	// CreateCommand
	cmd := &models.DeviceCommand{DeviceID: deviceID, CommandType: "lock"}
	require.NoError(t, svc.CreateCommand(ctx, cmd))
	assert.NotEqual(t, uuid.Nil, cmd.ID)

	// ListCommands
	cmds, total, err := svc.ListCommands(ctx, deviceID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, cmds, 1)

	// ListCertificates
	certs, total, err := svc.ListCertificates(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, certs)
}
