package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertificateRepository is the interface for certificate data access (read-only).
type CertificateRepository interface {
	List(ctx context.Context, deviceID *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error)
}

// FullCommandRepository extends CommandRepository with read methods needed by CommandService.
type FullCommandRepository interface {
	CommandRepository
	ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error)
}

// CommandService handles device command and certificate listing business logic.
type CommandService struct {
	cmdRepo    FullCommandRepository
	deviceRepo DeviceRepository
	certRepo   CertificateRepository
	logger     *slog.Logger
}

// NewCommandService creates a new command service.
func NewCommandService(cmdRepo FullCommandRepository, deviceRepo DeviceRepository, certRepo CertificateRepository, logger *slog.Logger) *CommandService {
	return &CommandService{
		cmdRepo:    cmdRepo,
		deviceRepo: deviceRepo,
		certRepo:   certRepo,
		logger:     logger,
	}
}

// GetDevice retrieves a device by ID (used for command validation).
func (s *CommandService) GetDevice(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get device %s: %w", id, err)
	}
	return device, nil
}

// CreateCommand creates a new device command.
func (s *CommandService) CreateCommand(ctx context.Context, cmd *models.DeviceCommand) error {
	if err := s.cmdRepo.Create(ctx, cmd); err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	return nil
}

// ListCommands returns commands for a device.
func (s *CommandService) ListCommands(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error) {
	return s.cmdRepo.ListByDevice(ctx, deviceID, limit, offset)
}

// ListCertificates returns certificates, optionally filtered by device.
func (s *CommandService) ListCertificates(ctx context.Context, deviceID *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error) {
	return s.certRepo.List(ctx, deviceID, limit, offset)
}
