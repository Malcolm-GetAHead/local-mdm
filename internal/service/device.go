package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// DeviceRepository is the interface used by DeviceService.
type DeviceRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error)
	Update(ctx context.Context, device *models.Device) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// CommandRepository is the interface used by DeviceService.
type CommandRepository interface {
	Create(ctx context.Context, cmd *models.DeviceCommand) error
}

// CommandDispatcher enqueues commands for async delivery to devices.
type CommandDispatcher interface {
	Enqueue(device *models.Device, cmd *models.DeviceCommand)
}

// DeviceService handles device business logic.
type DeviceService struct {
	deviceRepo  DeviceRepository
	cmdRepo     CommandRepository
	dispatcher  CommandDispatcher
	lifecycle   *LifecycleService
	logger      *slog.Logger
}

// NewDeviceService creates a new DeviceService.
func NewDeviceService(deviceRepo DeviceRepository, cmdRepo CommandRepository, dispatcher CommandDispatcher, lifecycle *LifecycleService, logger *slog.Logger) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		cmdRepo:    cmdRepo,
		dispatcher: dispatcher,
		lifecycle:  lifecycle,
		logger:     logger,
	}
}

// Get returns a device by ID.
func (s *DeviceService) Get(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	return s.deviceRepo.GetByID(ctx, id)
}

// List returns devices for an enterprise.
func (s *DeviceService) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	return s.deviceRepo.List(ctx, enterpriseID, limit, offset)
}

// Update applies partial updates to a device.
func (s *DeviceService) Update(ctx context.Context, id uuid.UUID, name, model, osVersion, status *string, platformData *models.JSONB) (*models.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		device.Name = *name
	}
	if model != nil {
		device.Model = *model
	}
	if osVersion != nil {
		device.OSVersion = *osVersion
	}
	if status != nil {
		device.Status = *status
	}
	if platformData != nil {
		device.PlatformData = *platformData
	}
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return nil, err
	}
	return device, nil
}

// Delete soft-deletes a device and fires lifecycle hooks.
func (s *DeviceService) Delete(ctx context.Context, id uuid.UUID) error {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// For macOS devices, send RemoveProfile to unenroll from MDM before soft-deleting.
	// The enrollment profile has CheckOutWhenRemoved=true, so the device will send a
	// CheckOut to NanoMDM on next check-in, completing the unenrollment.
	if device.Platform == models.PlatformMacOS {
		profileID := fmt.Sprintf("com.localmdm.%s", device.EnterpriseID.String())
		cmd := &models.DeviceCommand{
			DeviceID:    device.ID,
			CommandType: models.CommandTypeRemoveProfile,
			CommandData: models.JSONB{"identifier": profileID},
			Status:      models.CommandStatusPending,
		}
		if err := s.cmdRepo.Create(ctx, cmd); err != nil {
			s.logger.Warn("failed to create remove-profile command, proceeding with delete",
				"error", err, "device_id", id)
		} else {
			s.dispatcher.Enqueue(device, cmd)
		}
	}

	if err := s.deviceRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.lifecycle.OnDelete(ctx, device)
	return nil
}

// ActionResult holds the device and command from a remote action.
type ActionResult struct {
	Device  *models.Device
	Command *models.DeviceCommand
}

// Lock creates a lock command and updates device status to lost.
func (s *DeviceService) Lock(ctx context.Context, id uuid.UUID) (*ActionResult, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cmd := &models.DeviceCommand{DeviceID: device.ID, CommandType: models.CommandTypeLock}
	if err := s.cmdRepo.Create(ctx, cmd); err != nil {
		return nil, fmt.Errorf("failed to create lock command: %w", err)
	}
	device.Status = models.DeviceStatusLost
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device status: %w", err)
	}
	s.dispatcher.Enqueue(device, cmd)
	return &ActionResult{Device: device, Command: cmd}, nil
}

// Wipe creates a wipe command, updates status, and fires lifecycle hooks.
func (s *DeviceService) Wipe(ctx context.Context, id uuid.UUID) (*ActionResult, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cmd := &models.DeviceCommand{DeviceID: device.ID, CommandType: models.CommandTypeWipe}
	if err := s.cmdRepo.Create(ctx, cmd); err != nil {
		return nil, fmt.Errorf("failed to create wipe command: %w", err)
	}
	device.Status = models.DeviceStatusWiped
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device status: %w", err)
	}
	s.dispatcher.Enqueue(device, cmd)
	s.lifecycle.OnWipe(ctx, device)
	return &ActionResult{Device: device, Command: cmd}, nil
}

// Restart creates a restart command for the device.
func (s *DeviceService) Restart(ctx context.Context, id uuid.UUID) (*ActionResult, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(device.Platform, models.PlatformWindows) {
		return nil, fmt.Errorf("restart not supported on Windows devices")
	}
	cmd := &models.DeviceCommand{DeviceID: device.ID, CommandType: models.CommandTypeRestart}
	if err := s.cmdRepo.Create(ctx, cmd); err != nil {
		return nil, fmt.Errorf("failed to create restart command: %w", err)
	}
	s.dispatcher.Enqueue(device, cmd)
	return &ActionResult{Device: device, Command: cmd}, nil
}

// Unenroll marks a device as unenrolled and fires lifecycle hooks.
func (s *DeviceService) Unenroll(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	device.Status = "unenrolled"
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to update device status: %w", err)
	}
	if s.lifecycle != nil {
		s.lifecycle.OnUnenroll(ctx, device)
	}
	return device, nil
}
