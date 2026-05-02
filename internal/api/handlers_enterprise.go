package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Enterprise handlers

func (s *Server) handleListEnterprises(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	enterprises, total, err := s.enterpriseService.List(r.Context(), limit, offset)
	if err != nil {
		s.logger.Error("failed to list enterprises", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list enterprises")
		return
	}

	respondPaginated(w, r, http.StatusOK, enterprises, total, limit, offset)
}

func (s *Server) handleCreateEnterprise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string       `json:"name"`
		Slug     string       `json:"slug"`
		Settings models.JSONB `json:"settings"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	if strings.TrimSpace(req.Slug) == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "slug is required")
		return
	}

	enterprise := &models.Enterprise{
		Name:     strings.TrimSpace(req.Name),
		Slug:     strings.TrimSpace(req.Slug),
		Settings: req.Settings,
	}

	if err := s.enterpriseService.Create(r.Context(), enterprise); err != nil {
		if isDuplicateError(err) {
			respondError(w, r, http.StatusConflict, "conflict", "Enterprise with this slug already exists")
			return
		}
		s.logger.Error("failed to create enterprise", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create enterprise")
		return
	}

	s.logAudit(r, "enterprise.create", "enterprise", enterprise.ID, map[string]interface{}{
		"name": enterprise.Name,
		"slug": enterprise.Slug,
	})

	respondJSON(w, r, http.StatusCreated, enterprise)
}

func (s *Server) handleGetEnterprise(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	enterprise, err := s.enterpriseService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to get enterprise", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get enterprise")
		return
	}

	respondJSON(w, r, http.StatusOK, enterprise)
}

func (s *Server) handleUpdateEnterprise(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	enterprise, err := s.enterpriseService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to get enterprise", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get enterprise")
		return
	}

	var req struct {
		Name     *string       `json:"name"`
		Settings *models.JSONB `json:"settings"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "name cannot be empty")
			return
		}
		enterprise.Name = strings.TrimSpace(*req.Name)
	}
	if req.Settings != nil {
		enterprise.Settings = *req.Settings
	}

	if err := s.enterpriseService.Update(r.Context(), enterprise); err != nil {
		s.logger.Error("failed to update enterprise", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update enterprise")
		return
	}

	s.logAudit(r, "enterprise.update", "enterprise", id, map[string]interface{}{
		"name": enterprise.Name,
	})

	respondJSON(w, r, http.StatusOK, enterprise)
}

func (s *Server) handleDeleteEnterprise(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	if err := s.enterpriseService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to delete enterprise", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete enterprise")
		return
	}

	s.logAudit(r, "enterprise.delete", "enterprise", id, nil)

	w.WriteHeader(http.StatusNoContent)
}
