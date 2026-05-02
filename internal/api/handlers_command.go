package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Command & Profile handlers (Sprint 3)

func (s *Server) handleSendCommand(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.commandService.GetDevice(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	var req struct {
		CommandType string       `json:"command_type"`
		CommandData models.JSONB `json:"command_data,omitempty"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.CommandType == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "command_type is required")
		return
	}

	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: req.CommandType,
		CommandData: req.CommandData,
	}
	if err := s.commandService.CreateCommand(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create command", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create command")
		return
	}

	s.logAudit(r, "command.send", "device", deviceID, map[string]interface{}{
		"command_type": req.CommandType,
		"command_id":   cmd.ID,
	})

	respondJSON(w, r, http.StatusCreated, cmd)
}

func (s *Server) handleListCommands(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	limit, offset := parsePagination(r)
	cmds, total, err := s.commandService.ListCommands(r.Context(), deviceID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list commands", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list commands")
		return
	}

	respondPaginated(w, r, http.StatusOK, cmds, total, limit, offset)
}

func (s *Server) handleInstallProfile(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.commandService.GetDevice(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	var req struct {
		ProfileType string       `json:"profile_type"`
		ProfileData models.JSONB `json:"profile_data"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.ProfileType == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "profile_type is required")
		return
	}

	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: models.CommandTypeInstallProfile,
		CommandData: models.JSONB{
			"profile_type": req.ProfileType,
			"profile_data": req.ProfileData,
		},
	}
	if err := s.commandService.CreateCommand(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create install profile command", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to install profile")
		return
	}

	s.logAudit(r, "profile.install", "device", deviceID, map[string]interface{}{
		"profile_type": req.ProfileType,
		"command_id":   cmd.ID,
	})

	respondJSON(w, r, http.StatusCreated, cmd)
}

func (s *Server) handleRemoveProfile(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	profileID := mux.Vars(r)["profile_id"]
	if profileID == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "profile_id is required")
		return
	}

	device, err := s.commandService.GetDevice(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: models.CommandTypeRemoveProfile,
		CommandData: models.JSONB{
			"profile_identifier": profileID,
		},
	}
	if err := s.commandService.CreateCommand(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create remove profile command", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to remove profile")
		return
	}

	s.logAudit(r, "profile.remove", "device", deviceID, map[string]interface{}{
		"profile_identifier": profileID,
		"command_id":         cmd.ID,
	})

	respondJSON(w, r, http.StatusCreated, cmd)
}

// Certificate handlers
func (s *Server) handleListCertificates(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	// Optional device_id filter
	var deviceID *uuid.UUID
	if v := r.URL.Query().Get("device_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "invalid_device_id", "Invalid device_id format")
			return
		}
		deviceID = &id
	}

	certs, total, err := s.commandService.ListCertificates(r.Context(), deviceID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list certificates", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list certificates")
		return
	}

	respondPaginated(w, r, http.StatusOK, certs, total, limit, offset)
}
