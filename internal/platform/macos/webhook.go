package macos

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"howett.net/plist"
)

// WebhookEvent matches NanoMDM v0.9.0 webhook JSON format (EventJson)
type WebhookEvent struct {
	Topic            string            `json:"topic"`
	EventID          *string           `json:"event_id,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	CheckinEvent     *CheckinEvent     `json:"checkin_event,omitempty"`
	AcknowledgeEvent *AcknowledgeEvent `json:"acknowledge_event,omitempty"`
}

// CheckinEvent matches NanoMDM's checkin webhook payload
type CheckinEvent struct {
	UDID             *string           `json:"udid,omitempty"`
	EnrollmentID     *string           `json:"enrollment_id,omitempty"`
	IDs              *EnrollmentIDs    `json:"ids,omitempty"`
	RawPayload       string            `json:"raw_payload"`
	TokenUpdateTally *int              `json:"token_update_tally,omitempty"`
	URLParams        map[string]string `json:"url_params,omitempty"`
}

// AcknowledgeEvent matches NanoMDM's command result webhook payload
type AcknowledgeEvent struct {
	UDID         *string           `json:"udid,omitempty"`
	EnrollmentID *string           `json:"enrollment_id,omitempty"`
	IDs          *EnrollmentIDs    `json:"ids,omitempty"`
	CommandUUID  *string           `json:"command_uuid,omitempty"`
	Status       string            `json:"status"`
	RawPayload   string            `json:"raw_payload"`
	URLParams    map[string]string `json:"url_params,omitempty"`
}

// EnrollmentIDs from NanoMDM
type EnrollmentIDs struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id,omitempty"`
	Type     string  `json:"type"`
}

// authenticatePlist represents the Authenticate check-in plist from Apple devices
type authenticatePlist struct {
	MessageType  string `plist:"MessageType"`
	UDID         string `plist:"UDID"`
	Topic        string `plist:"Topic"`
	SerialNumber string `plist:"SerialNumber"`
	Model        string `plist:"Model"`
	ModelName    string `plist:"ModelName"`
	ProductName  string `plist:"ProductName"`
	OSVersion    string `plist:"OSVersion"`
	BuildVersion string `plist:"BuildVersion"`
	DeviceName   string `plist:"DeviceName"`
}

// tokenUpdatePlist represents the TokenUpdate check-in plist
type tokenUpdatePlist struct {
	MessageType string `plist:"MessageType"`
	UDID        string `plist:"UDID"`
	Topic       string `plist:"Topic"`
	PushMagic   string `plist:"PushMagic"`
	Token       []byte `plist:"Token"`
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

	// Extract message type from topic (e.g. "mdm.Authenticate" -> "Authenticate")
	messageType := ""
	if idx := strings.LastIndex(event.Topic, "."); idx >= 0 {
		messageType = event.Topic[idx+1:]
	}

	// Get UDID from checkin event or acknowledge event
	udid := ""
	if event.CheckinEvent != nil && event.CheckinEvent.UDID != nil {
		udid = string(*event.CheckinEvent.UDID)
	}
	if event.AcknowledgeEvent != nil && event.AcknowledgeEvent.UDID != nil {
		udid = string(*event.AcknowledgeEvent.UDID)
	}

	h.logger.Info("mdm webhook",
		"topic", event.Topic,
		"message_type", messageType,
		"udid", udid,
	)

	if event.CheckinEvent != nil {
		if err := h.nanomdm.HandleCheckin(r.Context(), udid, messageType); err != nil {
			h.logger.Error("nanomdm checkin failed", "error", err, "udid", udid)
		}
		h.handleCheckin(r.Context(), messageType, udid, event.CheckinEvent)
	}

	if event.AcknowledgeEvent != nil {
		h.handleAcknowledge(r.Context(), event.AcknowledgeEvent)
	}

	// Update last_seen on any event with a UDID
	if udid != "" {
		if device, err := h.service.GetDeviceByUDID(r.Context(), udid); err == nil {
			now := time.Now()
			device.LastSeen = &now
			_ = h.service.UpdateDevice(r.Context(), device)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *CheckinHandler) handleCheckin(ctx context.Context, messageType, udid string, ce *CheckinEvent) {
	switch messageType {
	case "Authenticate":
		if udid == "" {
			return
		}
		// Parse the raw plist to extract device info
		h.logger.Debug("authenticate raw payload", "payload_len", len(ce.RawPayload), "payload_prefix", ce.RawPayload[:min(200, len(ce.RawPayload))])
		var auth authenticatePlist
		serial := ""
		name := ""
		model := ""
		osVersion := ""
		buildVersion := ""
		topic := ""
		if _, err := plist.Unmarshal([]byte(ce.RawPayload), &auth); err != nil {
			h.logger.Warn("failed to parse Authenticate plist, creating device with UDID only", "error", err, "payload_len", len(ce.RawPayload))
		} else {
			serial = auth.SerialNumber
			name = auth.DeviceName
			model = auth.ModelName
			if model == "" {
				model = auth.ProductName
			}
			if model == "" {
				model = auth.Model
			}
			osVersion = auth.OSVersion
			buildVersion = auth.BuildVersion
			topic = auth.Topic
		}

		// Use a default enterprise ID
		enterpriseID, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")

		// Try to update existing device, or create new one
		device, err := h.service.GetDeviceByUDID(ctx, udid)
		if err != nil {
			// Device doesn't exist — create it
			newDevice, createErr := h.service.CreateDevice(ctx, enterpriseID, udid, serial)
			if createErr != nil {
				h.logger.Error("failed to create device on authenticate", "error", createErr, "udid", udid)
				return
			}
			device = newDevice
		}

		device.Name = name
		device.Model = model
		device.OSVersion = osVersion
		device.SerialNumber = serial
		if device.PlatformData == nil {
			device.PlatformData = models.JSONB{}
		}
		device.PlatformData["build_version"] = buildVersion
		device.PlatformData["topic"] = topic

		if err := h.service.UpdateDevice(ctx, device); err != nil {
			h.logger.Error("failed to update device on authenticate", "error", err, "udid", udid)
		}

		h.logger.Info("device authenticated",
			"udid", udid,
			"serial", serial,
			"model", model,
			"os_version", osVersion,
		)

	case "TokenUpdate":
		if udid == "" {
			return
		}
		// Skip user-channel TokenUpdates
		if ce.IDs != nil && ce.IDs.Type == "User" {
			return
		}

		device, err := h.service.GetDeviceByUDID(ctx, udid)
		if err != nil {
			h.logger.Warn("device not found for token update", "udid", udid, "error", err)
			return
		}

		// Parse TokenUpdate plist for push_magic
		var tu tokenUpdatePlist
		if _, err := plist.Unmarshal([]byte(ce.RawPayload), &tu); err == nil {
			if device.PlatformData == nil {
				device.PlatformData = models.JSONB{}
			}
			if tu.PushMagic != "" {
				device.PlatformData["push_magic"] = tu.PushMagic
			}
			device.PlatformData["has_token"] = true
		}

		device.Status = models.DeviceStatusEnrolled
		if err := h.service.UpdateDevice(ctx, device); err != nil {
			h.logger.Error("failed to update device on token update", "error", err, "udid", udid)
		} else {
			h.logger.Info("device enrolled", "udid", udid, "device_id", device.ID)
		}

	case "CheckOut":
		if udid == "" {
			return
		}
		device, err := h.service.GetDeviceByUDID(ctx, udid)
		if err != nil {
			h.logger.Warn("device not found for checkout", "udid", udid, "error", err)
			return
		}
		device.Status = models.DeviceStatusUnenrolled
		if err := h.service.UpdateDevice(ctx, device); err != nil {
			h.logger.Error("failed to update device status on checkout", "error", err, "udid", udid)
		}
		if h.lifecycle != nil {
			h.lifecycle.OnUnenroll(ctx, device)
		}
		h.logger.Info("device unenrolled via checkout", "udid", udid, "device_id", device.ID)
	}
}

func (h *CheckinHandler) handleAcknowledge(ctx context.Context, ae *AcknowledgeEvent) {
	udid := ""
	if ae.UDID != nil {
		udid = string(*ae.UDID)
	}
	cmdUUID := ""
	if ae.CommandUUID != nil {
		cmdUUID = *ae.CommandUUID
	}

	h.logger.Info("mdm command result",
		"udid", udid,
		"command_uuid", cmdUUID,
		"status", ae.Status,
	)

	if err := h.nanomdm.HandleCommand(ctx, udid, cmdUUID, ae.Status); err != nil {
		h.logger.Error("nanomdm command handling failed", "error", err, "udid", udid)
	}
}

func parseUUID(s string) ([16]byte, error) {
	id, err := uuid.Parse(s)
	return id, err
}

// CommandHandler handles direct MDM command requests (NanoMDM proxy)
type CommandHandler struct {
	nanomdm *NanoMDMService
	logger  *slog.Logger
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(nanomdm *NanoMDMService, logger *slog.Logger) *CommandHandler {
	return &CommandHandler{nanomdm: nanomdm, logger: logger}
}

// ServeHTTP proxies MDM command requests
func (h *CommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
