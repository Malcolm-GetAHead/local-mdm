package macos

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// Service handles macOS MDM operations
type Service struct {
	deviceRepo repository.DeviceRepository
}

// NewService creates a new macOS MDM service
func NewService(deviceRepo repository.DeviceRepository) *Service {
	return &Service{
		deviceRepo: deviceRepo,
	}
}

// CreateDevice creates a new macOS device record
func (s *Service) CreateDevice(ctx context.Context, enterpriseID uuid.UUID, udid, serialNumber string) (*models.Device, error) {
	device := &models.Device{
		EnterpriseID: enterpriseID,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: serialNumber,
		Status:       models.DeviceStatusPending,
		PlatformData: make(models.JSONB),
	}

	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return device, nil
}

// UpdateDeviceStatus updates device status and last seen timestamp
func (s *Service) UpdateDeviceStatus(ctx context.Context, deviceID uuid.UUID, status string) error {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	device.Status = status
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}

	return nil
}

// GetDeviceByUDID finds a macOS device by its UDID (stored as device_id).
func (s *Service) GetDeviceByUDID(ctx context.Context, udid string) (*models.Device, error) {
	return s.deviceRepo.GetByPlatformID(ctx, models.PlatformMacOS, udid)
}

// GetDeviceByUDIDIncludeDeleted finds a macOS device by UDID, including soft-deleted records.
func (s *Service) GetDeviceByUDIDIncludeDeleted(ctx context.Context, udid string) (*models.Device, error) {
	return s.deviceRepo.GetByPlatformIDIncludeDeleted(ctx, models.PlatformMacOS, udid)
}

// UpdateDevice updates a device record.
func (s *Service) UpdateDevice(ctx context.Context, device *models.Device) error {
	return s.deviceRepo.Update(ctx, device)
}
