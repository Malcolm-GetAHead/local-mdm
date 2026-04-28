package macos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
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
	nanomdm       *NanoMDMService
	service       *Service
	cmdRepo       repository.CommandRepository
	lifecycle     LifecycleNotifier
	logger        *slog.Logger
	// Auto-query cooldown: track last auto-queue time per UDID to prevent storm cycles
	lastAutoQuery map[string]time.Time
	autoQueryMu   sync.Mutex
}

// NewCheckinHandler creates a new check-in handler
func NewCheckinHandler(nanomdm *NanoMDMService, service *Service, cmdRepo repository.CommandRepository, lifecycle LifecycleNotifier, logger *slog.Logger) *CheckinHandler {
	return &CheckinHandler{
		nanomdm:       nanomdm,
		service:       service,
		cmdRepo:       cmdRepo,
		lifecycle:     lifecycle,
		logger:        logger,
		lastAutoQuery: make(map[string]time.Time),
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

	// Auto-queue device info commands on Idle (no pending commands)
	// Only if we haven't auto-queued for this device recently (15min cooldown)
	if event.AcknowledgeEvent != nil && event.AcknowledgeEvent.Status == "Idle" && udid != "" {
		h.maybeAutoQueue(r.Context(), udid)
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
		decoded, decErr := base64.StdEncoding.DecodeString(ce.RawPayload)
		if decErr != nil {
			decoded = []byte(ce.RawPayload) // fallback to raw
		}
		if _, err := plist.Unmarshal(decoded, &auth); err != nil {
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
		tuDecoded, tuDecErr := base64.StdEncoding.DecodeString(ce.RawPayload)
		if tuDecErr != nil {
			tuDecoded = []byte(ce.RawPayload)
		}
		if _, err := plist.Unmarshal(tuDecoded, &tu); err == nil {
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

	// Parse command results and update device platform_data
	if ae.Status == "Acknowledged" && udid != "" && ae.RawPayload != "" {
		h.logger.Debug("raw acknowledge payload", "len", len(ae.RawPayload), "prefix", ae.RawPayload[:min(300, len(ae.RawPayload))])
		h.processCommandResult(ctx, udid, ae.RawPayload)
		// Mark command as completed in Local MDM
		if cmdUUID != "" && h.cmdRepo != nil {
			if cmdID, err := uuid.Parse(cmdUUID); err == nil {
				_ = h.cmdRepo.MarkCompleted(ctx, cmdID)
			}
		}
	}
}

// commandResultPlist is a generic plist response from MDM commands
type commandResultPlist struct {
	CommandUUID              string                   `plist:"CommandUUID"`
	Status                   string                   `plist:"Status"`
	SecurityInfo             map[string]interface{}   `plist:"SecurityInfo"`
	QueryResponses           map[string]interface{}   `plist:"QueryResponses"`
	ProfileList              []map[string]interface{} `plist:"ProfileList"`
	InstalledApplicationList []map[string]interface{} `plist:"InstalledApplicationList"`
}

func (h *CheckinHandler) processCommandResult(ctx context.Context, udid, rawPayload string) {
	device, err := h.service.GetDeviceByUDID(ctx, udid)
	if err != nil {
		return
	}
	if device.PlatformData == nil {
		device.PlatformData = models.JSONB{}
	}

	// raw_payload is base64-encoded by NanoMDM
	decoded, err := base64.StdEncoding.DecodeString(rawPayload)
	if err != nil {
		h.logger.Warn("failed to base64-decode raw_payload", "error", err)
		return
	}

	var result commandResultPlist
	if _, err := plist.Unmarshal(decoded, &result); err != nil {
		h.logger.Warn("failed to parse command result plist", "error", err)
		return
	}

	updated := false

	// SecurityInfo — compliance-relevant data
	if result.SecurityInfo != nil {
		si := result.SecurityInfo
		if v, ok := si["FDE_Enabled"]; ok {
			device.PlatformData["FileVaultEnabled"] = v
			device.PlatformData["encryption_enabled"] = v
			updated = true
		}
		if fw, ok := si["FirewallSettings"].(map[string]interface{}); ok {
			if enabled, ok := fw["FirewallEnabled"]; ok {
				device.PlatformData["firewall_enabled"] = enabled
			} else {
				device.PlatformData["firewall_enabled"] = true
			}
			updated = true
		}
		if v, ok := si["AuthenticatedRootVolumeEnabled"]; ok {
			device.PlatformData["authenticated_root_volume"] = v
			updated = true
		}
		if v, ok := si["IsActivationLockManageable"]; ok {
			device.PlatformData["activation_lock_manageable"] = v
			updated = true
		}
		if v, ok := si["ExternalBootLevel"]; ok {
			device.PlatformData["external_boot_level"] = v
			updated = true
		}
		if v, ok := si["SecureBoot"]; ok {
			device.PlatformData["secure_boot"] = v
			updated = true
		}
		h.logger.Info("security info processed", "udid", udid,
			"filevault", device.PlatformData["FileVaultEnabled"],
			"firewall", device.PlatformData["firewall_enabled"])
	}

	// DeviceInformation QueryResponses
	if result.QueryResponses != nil {
		qr := result.QueryResponses
		if v, ok := qr["DeviceName"].(string); ok && v != "" {
			device.Name = v
			updated = true
		}
		if v, ok := qr["OSVersion"].(string); ok && v != "" {
			device.OSVersion = v
			updated = true
		}
		if v, ok := qr["BuildVersion"].(string); ok && v != "" {
			device.PlatformData["build_version"] = v
			updated = true
		}
		if v, ok := qr["ModelName"].(string); ok && v != "" {
			device.Model = v
			updated = true
		}
		if v, ok := qr["SerialNumber"].(string); ok && v != "" {
			device.SerialNumber = v
			updated = true
		}
		if v, ok := qr["WiFiMAC"].(string); ok {
			device.PlatformData["wifi_mac"] = v
			updated = true
		}
		if v, ok := qr["BluetoothMAC"].(string); ok {
			device.PlatformData["bluetooth_mac"] = v
			updated = true
		}
		if v, ok := qr["IsSupervised"]; ok {
			device.PlatformData["is_supervised"] = v
			updated = true
		}
		if v, ok := qr["DeviceCapacity"]; ok {
			device.PlatformData["storage_capacity_gb"] = v
			updated = true
		}
		if v, ok := qr["AvailableDeviceCapacity"]; ok {
			device.PlatformData["storage_available_gb"] = v
			updated = true
		}
		if v, ok := qr["BatteryLevel"]; ok {
			device.PlatformData["battery_level"] = v
			updated = true
		}
		if v, ok := qr["HardwareEncryptionCaps"]; ok {
			device.PlatformData["hardware_encryption_caps"] = v
			updated = true
		}
		if v, ok := qr["HostName"].(string); ok {
			device.PlatformData["hostname"] = v
			updated = true
		}
		h.logger.Info("device info processed", "udid", udid, "device_name", device.Name, "os_version", device.OSVersion)
	}

	// ProfileList — store details
	if result.ProfileList != nil {
		profiles := make([]map[string]interface{}, 0, len(result.ProfileList))
		for _, p := range result.ProfileList {
			profile := map[string]interface{}{
				"name":         p["PayloadDisplayName"],
				"identifier":   p["PayloadIdentifier"],
				"organization": p["PayloadOrganization"],
				"uuid":         p["PayloadUUID"],
				"is_managed":   p["IsManaged"],
			}
			// Count sub-payloads
			if content, ok := p["PayloadContent"].([]interface{}); ok {
				profile["payload_count"] = len(content)
			}
			profiles = append(profiles, profile)
		}
		device.PlatformData["installed_profiles"] = profiles
		device.PlatformData["installed_profiles_count"] = len(profiles)
		updated = true
		h.logger.Info("profile list processed", "udid", udid, "count", len(profiles))
	}

	// InstalledApplicationList — store details
	if result.InstalledApplicationList != nil {
		apps := make([]map[string]interface{}, 0, len(result.InstalledApplicationList))
		for _, a := range result.InstalledApplicationList {
			app := map[string]interface{}{
				"name":       a["Name"],
				"identifier": a["Identifier"],
				"version":    a["ShortVersion"],
			}
			if size, ok := a["BundleSize"]; ok {
				app["bundle_size"] = size
			}
			apps = append(apps, app)
		}
		device.PlatformData["installed_apps"] = apps
		device.PlatformData["installed_apps_count"] = len(apps)
		updated = true
		h.logger.Info("app list processed", "udid", udid, "count", len(apps))
	}

	if updated {
		if err := h.service.UpdateDevice(ctx, device); err != nil {
			h.logger.Error("failed to update device with command results", "error", err, "udid", udid)
		}
	}
}

// autoQueryCooldown is the minimum time between auto-queued info commands per device.
// Prevents storm cycles: device checks in → auto-queue → APNs push → device checks in → ...
const autoQueryCooldown = 15 * time.Minute

// maybeAutoQueue enqueues SecurityInfo + DeviceInformation if the cooldown has elapsed.
func (h *CheckinHandler) maybeAutoQueue(ctx context.Context, udid string) {
	h.autoQueryMu.Lock()
	last, ok := h.lastAutoQuery[udid]
	if ok && time.Since(last) < autoQueryCooldown {
		h.autoQueryMu.Unlock()
		return
	}
	h.lastAutoQuery[udid] = time.Now()
	h.autoQueryMu.Unlock()

	// Look up the device to get its ID and enterprise ID for command records
	device, err := h.service.GetDeviceByUDID(ctx, udid)
	if err != nil {
		return
	}

	h.logger.Info("auto-queuing device info commands", "udid", udid)

	type autoCmd struct {
		name    string
		cmdType string
		plist   string
	}

	commands := []autoCmd{
		{"SecurityInfo", "device_info", `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CommandUUID</key><string>auto-sec-%s</string><key>Command</key><dict><key>RequestType</key><string>SecurityInfo</string></dict></dict></plist>`},
		{"DeviceInformation", "device_info", `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CommandUUID</key><string>auto-dev-%s</string><key>Command</key><dict><key>RequestType</key><string>DeviceInformation</string><key>Queries</key><array><string>DeviceName</string><string>OSVersion</string><string>BuildVersion</string><string>ModelName</string><string>Model</string><string>SerialNumber</string><string>WiFiMAC</string><string>DeviceCapacity</string><string>AvailableDeviceCapacity</string><string>IsSupervised</string><string>HostName</string></array></dict></dict></plist>`},
		{"ProfileList", "device_info", `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CommandUUID</key><string>auto-prof-%s</string><key>Command</key><dict><key>RequestType</key><string>ProfileList</string></dict></dict></plist>`},
		{"InstalledApplicationList", "device_info", `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CommandUUID</key><string>auto-apps-%s</string><key>Command</key><dict><key>RequestType</key><string>InstalledApplicationList</string></dict></dict></plist>`},
	}

	now := time.Now()
	for _, cmd := range commands {
		cmdUUID := uuid.New()
		plistData := fmt.Sprintf(cmd.plist, cmdUUID.String())

		// Create command record in Local MDM
		if h.cmdRepo != nil {
			dbCmd := &models.DeviceCommand{
				BaseModel:    models.BaseModel{ID: cmdUUID},
				DeviceID:     device.ID,
				EnterpriseID: &device.EnterpriseID,
				CommandType:  cmd.cmdType,
				CommandData:  models.JSONB{"request_type": cmd.name, "auto_queued": true},
				Status:       "sent",
				SentAt:       &now,
			}
			if err := h.cmdRepo.Create(ctx, dbCmd); err != nil {
				h.logger.Error("failed to create command record", "error", err, "command", cmd.name)
			}
		}

		// Send to NanoMDM
		if _, err := h.nanomdm.SendCommand(ctx, udid, []byte(plistData)); err != nil {
			h.logger.Error("failed to auto-queue command", "error", err, "command", cmd.name, "udid", udid)
		}
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
