package service

import (
	"context"
	"log/slog"

	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// DeviceLifecycleHook is called on device lifecycle events.
// Implementations can perform cleanup, sync with external systems, etc.
type DeviceLifecycleHook interface {
	OnUnenroll(ctx context.Context, device *models.Device) error
	OnWipe(ctx context.Context, device *models.Device) error
	OnDelete(ctx context.Context, device *models.Device) error
}

// LifecycleService manages device lifecycle hooks.
type LifecycleService struct {
	hooks  []DeviceLifecycleHook
	logger *slog.Logger
}

// NewLifecycleService creates a new lifecycle service.
func NewLifecycleService(logger *slog.Logger) *LifecycleService {
	return &LifecycleService{logger: logger}
}

// RegisterHook adds a lifecycle hook.
func (s *LifecycleService) RegisterHook(hook DeviceLifecycleHook) {
	s.hooks = append(s.hooks, hook)
}

// OnUnenroll calls all registered hooks for device unenrollment.
func (s *LifecycleService) OnUnenroll(ctx context.Context, device *models.Device) {
	for _, h := range s.hooks {
		if err := h.OnUnenroll(ctx, device); err != nil {
			s.logger.Error("lifecycle hook OnUnenroll failed",
				"error", err, "device_id", device.ID, "platform", device.Platform)
		}
	}
}

// OnWipe calls all registered hooks for device wipe.
func (s *LifecycleService) OnWipe(ctx context.Context, device *models.Device) {
	for _, h := range s.hooks {
		if err := h.OnWipe(ctx, device); err != nil {
			s.logger.Error("lifecycle hook OnWipe failed",
				"error", err, "device_id", device.ID, "platform", device.Platform)
		}
	}
}

// OnDelete calls all registered hooks for device deletion.
func (s *LifecycleService) OnDelete(ctx context.Context, device *models.Device) {
	for _, h := range s.hooks {
		if err := h.OnDelete(ctx, device); err != nil {
			s.logger.Error("lifecycle hook OnDelete failed",
				"error", err, "device_id", device.ID, "platform", device.Platform)
		}
	}
}
