package windows

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// ManagementHandler processes OMA-DM sync sessions from enrolled Windows devices.
type ManagementHandler struct {
	serverURI  string
	deviceRepo repository.DeviceRepository
	cmdRepo    repository.CommandRepository
	logger     *slog.Logger
}

// NewManagementHandler creates a new OMA-DM management handler.
func NewManagementHandler(serverURI string, deviceRepo repository.DeviceRepository, cmdRepo repository.CommandRepository, logger *slog.Logger) *ManagementHandler {
	return &ManagementHandler{
		serverURI:  serverURI,
		deviceRepo: deviceRepo,
		cmdRepo:    cmdRepo,
		logger:     logger,
	}
}

// HandleSyncML processes an incoming SyncML message and returns a response.
// This implements the OMA-DM pkg 1 (client) → pkg 2 (server) exchange.
func (h *ManagementHandler) HandleSyncML(ctx context.Context, data []byte) ([]byte, error) {
	msg, err := ParseSyncML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SyncML: %w", err)
	}

	deviceID := msg.GetDeviceID()
	if deviceID == "" {
		return nil, fmt.Errorf("missing device ID in SyncML source")
	}

	h.logger.Info("OMA-DM sync received",
		"device_id", deviceID,
		"session_id", msg.SyncHdr.SessionID,
		"msg_id", msg.SyncHdr.MsgID,
	)

	// Build response
	resp := NewSyncMLResponse(
		msg.SyncHdr.SessionID,
		msg.SyncHdr.MsgID,
		h.serverURI,
		deviceID,
	)

	cmdID := 1

	// Status for SyncHdr (acknowledge the session)
	resp.AddStatus(strconv.Itoa(cmdID), msg.SyncHdr.MsgID, "0", "SyncHdr", StatusAuthAccepted)
	cmdID++

	// Process client statuses (acknowledgments of our previous commands)
	for _, status := range msg.SyncBody.Status {
		h.processStatus(ctx, deviceID, &status)
	}

	// Process client results (responses to our Get commands)
	for _, result := range msg.SyncBody.Results {
		h.processResults(ctx, deviceID, &result)
	}

	// Acknowledge client alerts
	for _, alert := range msg.SyncBody.Alert {
		resp.AddStatus(strconv.Itoa(cmdID), msg.SyncHdr.MsgID, alert.CmdID, "Alert", StatusOK)
		cmdID++
	}

	// Acknowledge client Replace commands (device reporting data)
	for _, replace := range msg.SyncBody.Replace {
		resp.AddStatus(strconv.Itoa(cmdID), msg.SyncHdr.MsgID, replace.CmdID, "Replace", StatusOK)
		cmdID++
		h.processReplace(ctx, deviceID, &replace)
	}

	// Dequeue and deliver pending commands
	cmdID, err = h.deliverPendingCommands(ctx, deviceID, resp, cmdID)
	if err != nil {
		h.logger.Error("failed to deliver pending commands", "error", err, "device_id", deviceID)
	}

	return GenerateSyncML(resp)
}

// processStatus handles status responses from the device for previously sent commands.
func (h *ManagementHandler) processStatus(ctx context.Context, deviceID string, status *SyncMLStatus) {
	// Status for commands we sent — update command queue
	if status.CmdRef == "0" {
		return // Status for SyncHdr, skip
	}

	h.logger.Info("device status response",
		"device_id", deviceID,
		"cmd_ref", status.CmdRef,
		"cmd", status.Cmd,
		"status", status.Data,
	)
}

// processResults handles results from Get commands (device info queries).
func (h *ManagementHandler) processResults(ctx context.Context, deviceID string, results *SyncMLResults) {
	for _, item := range results.Item {
		uri := ""
		if item.Source != nil {
			uri = item.Source.LocURI
		} else if item.Target != nil {
			uri = item.Target.LocURI
		}

		h.logger.Info("device result",
			"device_id", deviceID,
			"uri", uri,
			"data", item.Data,
		)

		// Update device record with reported info
		h.updateDeviceInfo(ctx, deviceID, uri, item.Data)
	}
}

// processReplace handles Replace commands from the device (unsolicited data reporting).
func (h *ManagementHandler) processReplace(ctx context.Context, deviceID string, replace *SyncMLReplace) {
	for _, item := range replace.Item {
		uri := ""
		if item.Source != nil {
			uri = item.Source.LocURI
		} else if item.Target != nil {
			uri = item.Target.LocURI
		}
		h.updateDeviceInfo(ctx, deviceID, uri, item.Data)
	}
}

// updateDeviceInfo updates the device's PlatformData with info from OMA-DM results.
func (h *ManagementHandler) updateDeviceInfo(ctx context.Context, deviceID, uri, value string) {
	if uri == "" || value == "" {
		return
	}

	// Map OMA-DM URIs to device fields
	fieldMap := map[string]string{
		"./DevDetail/Ext/Microsoft/DeviceName":              "device_name",
		"./DevDetail/Ext/Microsoft/OSPlatform":              "os_platform",
		"./DevDetail/Ext/Microsoft/ProcessorArchitecture":   "processor_arch",
		"./DevDetail/Ext/Microsoft/TotalRAM":                "total_ram",
		"./DevDetail/Ext/Microsoft/TotalStorage":            "total_storage",
		"./DevDetail/FwV":                                   "firmware_version",
		"./DevDetail/HwV":                                   "hardware_version",
		"./DevDetail/SwV":                                   "software_version",
		"./DevInfo/DevId":                                   "dev_id",
		"./DevInfo/Man":                                     "manufacturer",
		"./DevInfo/Mod":                                     "model",
		"./DevInfo/Lang":                                    "language",
	}

	field, ok := fieldMap[uri]
	if !ok {
		return
	}

	// Find device by device ID string and update PlatformData
	device, err := h.findDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		h.logger.Warn("device not found for info update", "device_id", deviceID, "error", err)
		return
	}

	if device.PlatformData == nil {
		device.PlatformData = models.JSONB{}
	}
	device.PlatformData[field] = value

	// Update model/name from specific fields
	if field == "model" {
		device.Model = value
	}
	if field == "device_name" {
		device.Name = value
	}
	if field == "software_version" {
		device.OSVersion = value
	}

	if err := h.deviceRepo.Update(ctx, device); err != nil {
		h.logger.Error("failed to update device info", "error", err, "device_id", deviceID, "field", field)
	}
}

// deliverPendingCommands dequeues pending commands and adds them to the response.
func (h *ManagementHandler) deliverPendingCommands(ctx context.Context, deviceID string, resp *SyncML, cmdID int) (int, error) {
	device, err := h.findDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return cmdID, err
	}

	cmds, err := h.cmdRepo.ListPending(ctx, device.ID)
	if err != nil {
		return cmdID, fmt.Errorf("failed to list pending commands: %w", err)
	}

	for _, cmd := range cmds {
		switch cmd.CommandType {
		case models.CommandTypeDeviceInfo:
			resp.AddGet(strconv.Itoa(cmdID), DevDetailNodes()...)
			cmdID++
		case models.CommandTypeLock:
			resp.AddExec(strconv.Itoa(cmdID), "./Vendor/MSFT/RemoteLock/Lock")
			cmdID++
		case models.CommandTypeWipe:
			resp.AddExec(strconv.Itoa(cmdID), "./Vendor/MSFT/RemoteWipe/doWipe")
			cmdID++
		default:
			h.logger.Warn("unknown command type", "type", cmd.CommandType)
			continue
		}

		if err := h.cmdRepo.MarkSent(ctx, cmd.ID); err != nil {
			h.logger.Error("failed to mark command sent", "error", err, "cmd_id", cmd.ID)
		}
	}

	return cmdID, nil
}

// findDeviceByDeviceID looks up a device by its OMA-DM device identifier string.
func (h *ManagementHandler) findDeviceByDeviceID(ctx context.Context, deviceID string) (*models.Device, error) {
	// Try parsing as UUID first (our internal device ID)
	if id, err := uuid.Parse(deviceID); err == nil {
		return h.deviceRepo.GetByID(ctx, id)
	}
	// Otherwise it's an OMA-DM device URI — not directly queryable yet
	// For now, return not found
	return nil, fmt.Errorf("device not found: %s", deviceID)
}

// DevDetailNodes returns the OMA-DM URIs for device detail queries.
func DevDetailNodes() []string {
	return []string{
		"./DevDetail/Ext/Microsoft/DeviceName",
		"./DevDetail/Ext/Microsoft/OSPlatform",
		"./DevDetail/Ext/Microsoft/ProcessorArchitecture",
		"./DevDetail/Ext/Microsoft/TotalRAM",
		"./DevDetail/Ext/Microsoft/TotalStorage",
		"./DevDetail/FwV",
		"./DevDetail/HwV",
		"./DevDetail/SwV",
		"./DevInfo/DevId",
		"./DevInfo/Man",
		"./DevInfo/Mod",
	}
}
