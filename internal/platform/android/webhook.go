package android

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"google.golang.org/api/androidmanagement/v1"
)

// WebhookHandler handles Android Management API webhooks
type WebhookHandler struct {
	service *Service
	client  *Client
	logger  *slog.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *Service, client *Client, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		service: service,
		client:  client,
		logger:  logger,
	}
}

// WebhookEvent represents an Android Management API webhook event
type WebhookEvent struct {
	NotificationType string                 `json:"notificationType"`
	EnterpriseToken  string                 `json:"enterpriseToken,omitempty"`
	DeviceName       string                 `json:"deviceName,omitempty"`
	UserName         string                 `json:"userName,omitempty"`
	Timestamp        string                 `json:"timestamp"`
	Data             map[string]interface{} `json:"data,omitempty"`
}

// HandleWebhook processes Android Management API webhook events
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var event WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.logger.Error("failed to decode webhook", "error", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	h.logger.Info("received webhook event",
		"type", event.NotificationType,
		"device", event.DeviceName,
	)

	switch event.NotificationType {
	case "ENROLLMENT":
		if err := h.handleEnrollment(ctx, &event); err != nil {
			h.logger.Error("failed to handle enrollment", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "COMPLIANCE_REPORT":
		if err := h.handleComplianceReport(ctx, &event); err != nil {
			h.logger.Error("failed to handle compliance report", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "STATUS_REPORT":
		if err := h.handleStatusReport(ctx, &event); err != nil {
			h.logger.Error("failed to handle status report", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "UNENROLLMENT":
		if err := h.handleUnenrollment(ctx, &event); err != nil {
			h.logger.Error("failed to handle unenrollment", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleEnrollment(ctx context.Context, event *WebhookEvent) error {
	if event.DeviceName == "" {
		return fmt.Errorf("missing device name in enrollment event")
	}

	h.logger.Info("device enrolled", "device_name", event.DeviceName)

	// Create device record if we have enough context
	// EnterpriseToken maps to our enterprise; DeviceName is the Google resource name
	if event.EnterpriseToken != "" {
		enterprise, err := h.service.enterpriseRepo.GetBySlug(ctx, event.EnterpriseToken)
		if err != nil {
			h.logger.Warn("cannot map enterprise token to enterprise",
				"enterprise_token", event.EnterpriseToken, "error", err)
			return nil
		}
		if _, err := h.service.CreateDevice(ctx, enterprise.ID, event.DeviceName, ""); err != nil {
			return fmt.Errorf("failed to create device: %w", err)
		}
	}

	return nil
}

func (h *WebhookHandler) handleComplianceReport(ctx context.Context, event *WebhookEvent) error {
	h.logger.Info("compliance report received",
		"device", event.DeviceName,
		"data", event.Data,
	)
	
	// Update device compliance status
	return nil
}

func (h *WebhookHandler) handleStatusReport(ctx context.Context, event *WebhookEvent) error {
	if event.DeviceName == "" {
		return nil
	}

	// Get device details (requires Google API client)
	if h.client == nil {
		h.logger.Warn("status report received but Google client not configured", "device", event.DeviceName)
		return nil
	}
	device, err := h.client.GetDevice(ctx, event.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to get device details: %w", err)
	}

	h.logger.Info("status report received",
		"device", event.DeviceName,
		"state", device.State,
		"applied_state", device.AppliedState,
	)

	// Update device status in database
	return nil
}

func (h *WebhookHandler) handleUnenrollment(ctx context.Context, event *WebhookEvent) error {
	h.logger.Info("device unenrolled", "device", event.DeviceName)

	// Look up device by device name (Google resource name stored as device_id)
	if event.DeviceName != "" && event.EnterpriseToken != "" {
		enterprise, err := h.service.enterpriseRepo.GetBySlug(ctx, event.EnterpriseToken)
		if err != nil {
			h.logger.Warn("cannot map enterprise for unenrollment",
				"enterprise_token", event.EnterpriseToken, "error", err)
			return nil
		}
		device, err := h.service.deviceRepo.GetBySerial(ctx, enterprise.ID, event.DeviceName)
		if err != nil {
			h.logger.Warn("device not found for unenrollment", "device_name", event.DeviceName)
			return nil
		}
		if err := h.service.UpdateDeviceStatus(ctx, device.ID, "unenrolled"); err != nil {
			return fmt.Errorf("failed to update device status: %w", err)
		}
	}

	return nil
}

// Poller handles periodic polling of device status
type Poller struct {
	service *Service
	client  *Client
	logger  *slog.Logger
}

// NewPoller creates a new device status poller
func NewPoller(service *Service, client *Client, logger *slog.Logger) *Poller {
	return &Poller{
		service: service,
		client:  client,
		logger:  logger,
	}
}

// Poll polls all enterprises for device status updates
func (p *Poller) Poll(ctx context.Context, enterpriseName string) error {
	devices, err := p.client.ListDevices(ctx, enterpriseName)
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	p.logger.Info("polled devices", "count", len(devices), "enterprise", enterpriseName)

	for _, device := range devices {
		if err := p.reconcileDevice(ctx, device); err != nil {
			p.logger.Error("failed to reconcile device",
				"device", device.Name,
				"error", err,
			)
			continue
		}
	}

	return nil
}

func (p *Poller) reconcileDevice(ctx context.Context, device *androidmanagement.Device) error {
	// Look up device in our database and update if needed
	// This is a backup mechanism in case webhooks were missed
	
	p.logger.Debug("reconciling device",
		"device", device.Name,
		"state", device.State,
	)

	return nil
}
