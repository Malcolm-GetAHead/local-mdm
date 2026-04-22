package api

import (
	"errors"
	"net/http"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
)

// Compliance handlers (Sprint 4)

func (s *Server) handleComplianceSummary(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	summary, err := s.complianceService.GetSummary(r.Context(), user.EnterpriseID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get compliance summary")
		return
	}
	respondJSON(w, r, http.StatusOK, summary)
}

func (s *Server) handleDeviceCompliance(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID")
		return
	}
	results, err := s.complianceService.GetDeviceCompliance(r.Context(), deviceID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get compliance")
		return
	}
	respondJSON(w, r, http.StatusOK, results)
}

func (s *Server) handleEvaluateDeviceCompliance(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID")
		return
	}
	device, err := s.deviceRepo.GetByID(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}
	results, err := s.complianceService.EvaluateDevice(r.Context(), deviceID, device.EnterpriseID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to evaluate compliance")
		return
	}
	s.logAudit(r, "compliance.evaluate", "device", deviceID, map[string]interface{}{"results": len(results)})
	respondJSON(w, r, http.StatusOK, results)
}
