package macos

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// NanoMDMService wraps NanoMDM functionality for sending commands and processing responses.
type NanoMDMService struct {
	cmdSender *CommandSender
	respProc  *ResponseProcessor
	logger    *slog.Logger
}

// NewNanoMDMService creates a new NanoMDM service instance.
// If nanomdmURL is empty, command sending is disabled (commands queue only).
func NewNanoMDMService(nanomdmURL, apiKey string, cmdRepo repository.CommandRepository, deviceRepo repository.DeviceRepository, logger *slog.Logger) *NanoMDMService {
	var sender *CommandSender
	if nanomdmURL != "" {
		sender = NewCommandSender(nanomdmURL, apiKey)
	}

	var respProc *ResponseProcessor
	if cmdRepo != nil {
		respProc = NewResponseProcessor(cmdRepo, deviceRepo, logger)
	}

	return &NanoMDMService{
		cmdSender: sender,
		respProc:  respProc,
		logger:    logger,
	}
}

// HandleCommand processes an MDM command result from NanoMDM webhook.
func (s *NanoMDMService) HandleCommand(ctx context.Context, udid, commandUUID, status string) error {
	s.logger.Info("handling mdm command result", "udid", udid, "command_uuid", commandUUID, "status", status)
	if s.respProc != nil {
		return s.respProc.ProcessCommandResult(ctx, udid, commandUUID, status)
	}
	return nil
}

// HandleCheckin processes an MDM check-in event.
func (s *NanoMDMService) HandleCheckin(ctx context.Context, udid string, messageType string) error {
	s.logger.Info("handling mdm checkin", "udid", udid, "message_type", messageType)
	return nil
}

// SendCommand sends a raw plist command to a device via NanoMDM.
// Returns nil if NanoMDM is not configured (commands will be delivered on next sync).
func (s *NanoMDMService) SendCommand(ctx context.Context, udid string, commandPlist []byte) (*EnqueueResponse, error) {
	if s.cmdSender == nil {
		s.logger.Info("NanoMDM not configured, command queued for next sync", "udid", udid)
		return nil, nil
	}
	return s.cmdSender.SendCommand(ctx, udid, commandPlist)
}

// HealthCheck verifies NanoMDM is reachable by hitting its /version endpoint.
func (s *NanoMDMService) HealthCheck(ctx context.Context) error {
	if s.cmdSender == nil {
		return fmt.Errorf("nanomdm not configured")
	}
	url := s.cmdSender.nanomdmURL + "/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("nanomdm unreachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("nanomdm returned status %d", resp.StatusCode)
	}
	return nil
}