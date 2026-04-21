package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// AppRepository is the interface used by AppService.
type AppRepository interface {
	Create(ctx context.Context, app *models.App) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.App, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.App, int, error)
	Update(ctx context.Context, app *models.App) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// AppService handles app business logic.
type AppService struct {
	appRepo    AppRepository
	deviceRepo DeviceRepository
	cmdRepo    CommandRepository
	dispatcher CommandDispatcher
	logger     *slog.Logger
}

// NewAppService creates a new AppService.
func NewAppService(appRepo AppRepository, deviceRepo DeviceRepository, cmdRepo CommandRepository, dispatcher CommandDispatcher, logger *slog.Logger) *AppService {
	return &AppService{
		appRepo:    appRepo,
		deviceRepo: deviceRepo,
		cmdRepo:    cmdRepo,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// Get returns an app by ID.
func (s *AppService) Get(ctx context.Context, id uuid.UUID) (*models.App, error) {
	return s.appRepo.GetByID(ctx, id)
}

// List returns apps for an enterprise.
func (s *AppService) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.App, int, error) {
	return s.appRepo.List(ctx, enterpriseID, limit, offset)
}

// Create validates and creates an app.
func (s *AppService) Create(ctx context.Context, app *models.App) error {
	if app.Name == "" || app.Platform == "" || app.Identifier == "" {
		return fmt.Errorf("name, platform, and identifier are required")
	}
	if app.InstallType == "" {
		app.InstallType = models.AppInstallRequired
	}
	return s.appRepo.Create(ctx, app)
}

// Update applies partial updates to an app.
func (s *AppService) Update(ctx context.Context, id uuid.UUID, name, version, installType *string, appConfig *models.JSONB) (*models.App, error) {
	app, err := s.appRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		app.Name = *name
	}
	if version != nil {
		app.Version = *version
	}
	if installType != nil {
		app.InstallType = *installType
	}
	if appConfig != nil {
		app.AppConfig = *appConfig
	}
	if err := s.appRepo.Update(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

// Delete removes an app.
func (s *AppService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.appRepo.Delete(ctx, id)
}

// DeployResult holds the results of a deploy operation.
type DeployResult struct {
	App      *models.App
	Commands []*models.DeviceCommand
}

// Deploy creates install commands for the given devices.
func (s *AppService) Deploy(ctx context.Context, appID uuid.UUID, deviceIDs []uuid.UUID) (*DeployResult, error) {
	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if len(deviceIDs) == 0 {
		return nil, fmt.Errorf("device_ids is required")
	}

	var commands []*models.DeviceCommand
	for _, deviceID := range deviceIDs {
		cmd := &models.DeviceCommand{
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallApp,
			CommandData: models.JSONB{
				"app_id":     app.ID,
				"identifier": app.Identifier,
				"name":       app.Name,
				"platform":   app.Platform,
			},
		}
		if err := s.cmdRepo.Create(ctx, cmd); err != nil {
			s.logger.Error("failed to create deploy command", "error", err, "device_id", deviceID)
			continue
		}
		if device, err := s.deviceRepo.GetByID(ctx, deviceID); err == nil {
			s.dispatcher.Enqueue(device, cmd)
		}
		commands = append(commands, cmd)
	}

	return &DeployResult{App: app, Commands: commands}, nil
}
