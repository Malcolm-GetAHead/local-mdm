package android

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// Service handles Android MDM operations
type Service struct {
	deviceRepo     repository.DeviceRepository
	enterpriseRepo repository.EnterpriseRepository
	projectID      string
	serviceAccount string
}

// NewService creates a new Android MDM service
func NewService(
	deviceRepo repository.DeviceRepository,
	enterpriseRepo repository.EnterpriseRepository,
	projectID string,
	serviceAccount string,
) *Service {
	return &Service{
		deviceRepo:     deviceRepo,
		enterpriseRepo: enterpriseRepo,
		projectID:      projectID,
		serviceAccount: serviceAccount,
	}
}

// CreateDevice creates a new Android device record
func (s *Service) CreateDevice(ctx context.Context, enterpriseID uuid.UUID, deviceID, serialNumber string) (*models.Device, error) {
	device := &models.Device{
		EnterpriseID: enterpriseID,
		Platform:     models.PlatformAndroid,
		DeviceID:     deviceID,
		SerialNumber: serialNumber,
		Status:       models.DeviceStatusPending,
		PlatformData: make(models.JSONB),
	}

	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return device, nil
}

// UpdateDeviceStatus updates device status
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
