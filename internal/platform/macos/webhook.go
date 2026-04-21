package macos

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// WebhookEvent represents a NanoMDM webhook event
type WebhookEvent struct {
	Topic        string          `json:"topic"`
	EventID      string          `json:"event_id"`
	EventAt      time.Time       `json:"event_at"`
	CheckinEvent *CheckinEvent   `json:"checkin_event,omitempty"`
}

// CheckinEvent represents device check-in data
type CheckinEvent struct {
	UDID         string                 `json:"udid"`
	EnrollmentID string                 `json:"enrollment_id,omitempty"`
	MessageType  string                 `json:"message_type"`
	Topic        string                 `json:"topic,omitempty"`
	Params       map[string]interface{} `json:"params,omitempty"`
}

// WebhookHandler handles NanoMDM webhook callbacks
type WebhookHandler struct {
	service *Service
	logger  *slog.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *Service, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		service: service,
		logger:  logger,
	}
}

// HandleWebhook processes NanoMDM webhook events
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var event WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.logger.Error("failed to decode webhook", "error", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	h.logger.Info("received webhook event",
		"topic", event.Topic,
		"event_id", event.EventID,
	)

	if event.CheckinEvent == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.logger.Info("processing checkin event",
		"message_type", event.CheckinEvent.MessageType,
		"udid", event.CheckinEvent.UDID,
	)

	switch event.CheckinEvent.MessageType {
	case "Authenticate":
		if err := h.handleAuthenticate(ctx, event.CheckinEvent); err != nil {
			h.logger.Error("failed to handle authenticate", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "TokenUpdate":
		if err := h.handleTokenUpdate(ctx, event.CheckinEvent); err != nil {
			h.logger.Error("failed to handle token update", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "CheckOut":
		if err := h.handleCheckOut(ctx, event.CheckinEvent); err != nil {
			h.logger.Error("failed to handle checkout", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleAuthenticate(ctx context.Context, event *CheckinEvent) error {
	h.logger.Info("device authenticated", "udid", event.UDID)
	return nil
}

func (h *WebhookHandler) handleTokenUpdate(ctx context.Context, event *CheckinEvent) error {
	h.logger.Info("device token updated", "udid", event.UDID)
	return nil
}

func (h *WebhookHandler) handleCheckOut(ctx context.Context, event *CheckinEvent) error {
	h.logger.Info("device checked out", "udid", event.UDID)
	return nil
}

// LifecycleNotifier is called on device lifecycle events.
type LifecycleNotifier interface {
	OnUnenroll(ctx context.Context, device *models.Device)
}

// CheckinHandler handles MDM check-in requests
type CheckinHandler struct {
	nanomdm   *NanoMDMService
	service   *Service
	lifecycle LifecycleNotifier
	logger    *slog.Logger
}

// NewCheckinHandler creates a new check-in handler
func NewCheckinHandler(nanomdm *NanoMDMService, service *Service, lifecycle LifecycleNotifier, logger *slog.Logger) *CheckinHandler {
	return &CheckinHandler{
		nanomdm:   nanomdm,
		service:   service,
		lifecycle: lifecycle,
		logger:    logger,
	}
}

// ServeHTTP handles MDM check-in HTTP requests (NanoMDM webhook JSON format)
func (h *CheckinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var event WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.logger.Error("failed to decode checkin webhook", "error", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if event.CheckinEvent == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	ce := event.CheckinEvent
	h.logger.Info("mdm checkin",
		"message_type", ce.MessageType,
		"udid", ce.UDID,
	)

	if err := h.nanomdm.HandleCheckin(r.Context(), ce.UDID, ce.MessageType); err != nil {
		h.logger.Error("nanomdm checkin failed", "error", err, "udid", ce.UDID)
	}

	// On Authenticate, create a device record if we have enterprise context
	if ce.MessageType == "Authenticate" && ce.UDID != "" {
		if enterpriseID, ok := ce.Params["enterprise_id"].(string); ok && enterpriseID != "" {
			if eid, err := parseUUID(enterpriseID); err == nil {
				if _, err := h.service.CreateDevice(r.Context(), eid, ce.UDID, ce.UDID); err != nil {
					h.logger.Error("failed to create device on authenticate", "error", err, "udid", ce.UDID)
				}
			}
		}
	}

	// On CheckOut, update device status and call lifecycle hooks
	if ce.MessageType == "CheckOut" && ce.UDID != "" {
		if device, err := h.service.GetDeviceByUDID(r.Context(), ce.UDID); err == nil {
			device.Status = models.DeviceStatusUnenrolled
			if err := h.service.UpdateDevice(r.Context(), device); err != nil {
				h.logger.Error("failed to update device status on checkout", "error", err, "udid", ce.UDID)
			}
			if h.lifecycle != nil {
				h.lifecycle.OnUnenroll(r.Context(), device)
			}
			h.logger.Info("device unenrolled via checkout", "udid", ce.UDID, "device_id", device.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func parseUUID(s string) ([16]byte, error) {
	id, err := uuid.Parse(s)
	return id, err
}

// CommandHandler handles MDM command requests
type CommandHandler struct {
	nanomdm *NanoMDMService
	logger  *slog.Logger
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(nanomdm *NanoMDMService, logger *slog.Logger) *CommandHandler {
	return &CommandHandler{
		nanomdm: nanomdm,
		logger:  logger,
	}
}

// CommandEvent represents a NanoMDM command result webhook event
type CommandEvent struct {
	UDID        string `json:"udid"`
	CommandUUID string `json:"command_uuid"`
	Status      string `json:"status"`
	RawPayload  string `json:"raw_payload,omitempty"`
}

// CommandWebhookEvent wraps a command result from NanoMDM
type CommandWebhookEvent struct {
	Topic        string        `json:"topic"`
	EventID      string        `json:"event_id"`
	CommandEvent *CommandEvent `json:"command_event,omitempty"`
}

// ServeHTTP handles MDM command HTTP requests (NanoMDM webhook JSON format)
func (h *CommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var event CommandWebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.logger.Error("failed to decode command webhook", "error", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if event.CommandEvent == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	ce := event.CommandEvent
	h.logger.Info("mdm command result",
		"udid", ce.UDID,
		"command_uuid", ce.CommandUUID,
		"status", ce.Status,
	)

	if err := h.nanomdm.HandleCommand(r.Context(), ce.UDID, ce.CommandUUID, ce.Status); err != nil {
		h.logger.Error("nanomdm command handling failed", "error", err, "udid", ce.UDID)
	}

	w.WriteHeader(http.StatusOK)
}
