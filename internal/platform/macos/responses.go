package macos

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// ResponseProcessor handles MDM command responses from NanoMDM webhooks.
type ResponseProcessor struct {
	cmdRepo    repository.CommandRepository
	deviceRepo repository.DeviceRepository
	logger     *slog.Logger
}

// NewResponseProcessor creates a new ResponseProcessor.
func NewResponseProcessor(cmdRepo repository.CommandRepository, deviceRepo repository.DeviceRepository, logger *slog.Logger) *ResponseProcessor {
	return &ResponseProcessor{
		cmdRepo:    cmdRepo,
		deviceRepo: deviceRepo,
		logger:     logger,
	}
}

// ProcessCommandResult handles a command result from NanoMDM.
// Status values from Apple: Acknowledged, Error, CommandFormatError, Idle, NotNow
func (p *ResponseProcessor) ProcessCommandResult(ctx context.Context, udid, commandUUID, status string) error {
	p.logger.Info("processing command result",
		"udid", udid,
		"command_uuid", commandUUID,
		"status", status,
	)

	cmdID, err := uuid.Parse(commandUUID)
	if err != nil {
		p.logger.Warn("non-UUID command_uuid, skipping DB update", "command_uuid", commandUUID)
		return nil
	}

	cmd, err := p.cmdRepo.GetByID(ctx, cmdID)
	if err != nil {
		p.logger.Warn("command not found in DB", "command_uuid", commandUUID, "error", err)
		return nil
	}

	switch status {
	case "Acknowledged":
		if err := p.cmdRepo.MarkCompleted(ctx, cmd.ID); err != nil {
			return fmt.Errorf("failed to mark command completed: %w", err)
		}
	case "Error", "CommandFormatError":
		if err := p.cmdRepo.MarkFailed(ctx, cmd.ID, fmt.Sprintf("device returned: %s", status)); err != nil {
			return fmt.Errorf("failed to mark command failed: %w", err)
		}
	case "NotNow":
		p.logger.Info("device deferred command", "command_uuid", commandUUID, "udid", udid)
		// Leave as sent — device will retry later
	default:
		p.logger.Warn("unknown command status", "status", status, "command_uuid", commandUUID)
	}

	return nil
}

// UpdateDeviceFromInfo updates a device record from DeviceInformation response data.
func (p *ResponseProcessor) UpdateDeviceFromInfo(ctx context.Context, udid string, info map[string]interface{}) error {
	devices, _, err := p.deviceRepo.List(ctx, uuid.Nil, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	var device *models.Device
	for _, d := range devices {
		if d.DeviceID == udid || d.SerialNumber == udid {
			device = d
			break
		}
	}
	if device == nil {
		return fmt.Errorf("device not found for UDID %s", udid)
	}

	if name, ok := info["DeviceName"].(string); ok && name != "" {
		device.Name = name
	}
	if osVer, ok := info["OSVersion"].(string); ok && osVer != "" {
		device.OSVersion = osVer
	}
	if model, ok := info["ModelName"].(string); ok && model != "" {
		device.Model = model
	}

	if device.PlatformData == nil {
		device.PlatformData = models.JSONB{}
	}
	for k, v := range info {
		device.PlatformData[k] = v
	}

	return p.deviceRepo.Update(ctx, device)
}
