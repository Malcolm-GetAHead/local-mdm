package api

import (
	"net/http"
	"strings"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Device handlers

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)

	devices, total, err := s.deviceService.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list devices", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list devices")
		return
	}

	respondPaginated(w, r, http.StatusOK, devices, total, limit, offset)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.deviceService.Get(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to get device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	respondJSON(w, r, http.StatusOK, device)
}

func (s *Server) handleLockDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	result, err := s.deviceService.Lock(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to lock device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to lock device")
		return
	}

	s.logAudit(r, "device.lock", "device", id, map[string]interface{}{
		"platform":   result.Device.Platform,
		"command_id": result.Command.ID,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  result.Device,
		"command": result.Command,
	})
}

func (s *Server) handleWipeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	result, err := s.deviceService.Wipe(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to wipe device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to wipe device")
		return
	}

	s.logAudit(r, "device.wipe", "device", id, map[string]interface{}{
		"platform":   result.Device.Platform,
		"command_id": result.Command.ID,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  result.Device,
		"command": result.Command,
	})
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	var req struct {
		Name         *string       `json:"name"`
		Model        *string       `json:"model"`
		OSVersion    *string       `json:"os_version"`
		Status       *string       `json:"status"`
		PlatformData *models.JSONB `json:"platform_data"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	device, err := s.deviceService.Update(r.Context(), id, req.Name, req.Model, req.OSVersion, req.Status, req.PlatformData)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to update device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update device")
		return
	}

	s.logAudit(r, "device.update", "device", id, map[string]interface{}{
		"platform": device.Platform,
	})

	respondJSON(w, r, http.StatusOK, device)
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	if err := s.deviceService.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to delete device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete device")
		return
	}

	s.logAudit(r, "device.delete", "device", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestartDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	result, err := s.deviceService.Restart(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		if strings.Contains(err.Error(), "not supported") {
			respondError(w, r, http.StatusBadRequest, "unsupported", err.Error())
			return
		}
		s.logger.Error("failed to restart device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to restart device")
		return
	}

	s.logAudit(r, "device.restart", "device", id, map[string]interface{}{
		"platform":   result.Device.Platform,
		"command_id": result.Command.ID,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  result.Device,
		"command": result.Command,
	})
}

func (s *Server) handleGetDeviceEffectivePolicies(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID")
		return
	}
	device, err := s.deviceRepo.GetByID(r.Context(), deviceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}
	assignments, err := s.groupService.GetEffectivePolicies(r.Context(), deviceID, device.EnterpriseID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get effective policies")
		return
	}
	respondJSON(w, r, http.StatusOK, assignments)
}
