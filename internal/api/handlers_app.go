package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// App Management handlers (Sprint 3)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)
	apps, total, err := s.appService.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list apps", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list apps")
		return
	}

	respondPaginated(w, r, http.StatusOK, apps, total, limit, offset)
}

func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req struct {
		Name        string       `json:"name"`
		Platform    string       `json:"platform"`
		Identifier  string       `json:"identifier"`
		Version     string       `json:"version"`
		InstallType string       `json:"install_type"`
		AppConfig   models.JSONB `json:"app_config"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if !isValidPlatform(req.Platform) {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "Invalid platform")
		return
	}

	app := &models.App{
		EnterpriseID: user.EnterpriseID,
		Name:         req.Name,
		Platform:     req.Platform,
		Identifier:   req.Identifier,
		Version:      req.Version,
		InstallType:  req.InstallType,
		AppConfig:    req.AppConfig,
	}

	if err := s.appService.Create(r.Context(), app); err != nil {
		if strings.Contains(err.Error(), "required") {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if isDuplicateError(err) {
			respondError(w, r, http.StatusConflict, "duplicate", "App already exists for this platform")
			return
		}
		s.logger.Error("failed to create app", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create app")
		return
	}

	s.logAudit(r, "app.create", "app", app.ID, map[string]interface{}{
		"name":       app.Name,
		"platform":   app.Platform,
		"identifier": app.Identifier,
	})

	respondJSON(w, r, http.StatusCreated, app)
}

func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid app ID format")
		return
	}

	app, err := s.appService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	respondJSON(w, r, http.StatusOK, app)
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid app ID format")
		return
	}

	var req struct {
		Name        *string       `json:"name"`
		Version     *string       `json:"version"`
		InstallType *string       `json:"install_type"`
		AppConfig   *models.JSONB `json:"app_config"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	app, err := s.appService.Update(r.Context(), id, req.Name, req.Version, req.InstallType, req.AppConfig)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		s.logger.Error("failed to update app", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update app")
		return
	}

	s.logAudit(r, "app.update", "app", id, nil)
	respondJSON(w, r, http.StatusOK, app)
}

func (s *Server) handleDeleteApp(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid app ID format")
		return
	}

	if err := s.appService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		s.logger.Error("failed to delete app", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete app")
		return
	}

	s.logAudit(r, "app.delete", "app", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeployApp(w http.ResponseWriter, r *http.Request) {
	appID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid app ID format")
		return
	}

	var req struct {
		DeviceIDs []uuid.UUID `json:"device_ids"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	result, err := s.appService.Deploy(r.Context(), appID, req.DeviceIDs)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		if strings.Contains(err.Error(), "required") {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		s.logger.Error("failed to deploy app", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to deploy app")
		return
	}

	s.logAudit(r, "app.deploy", "app", appID, map[string]interface{}{
		"device_ids": req.DeviceIDs,
		"commands":   len(result.Commands),
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"app":      result.App,
		"commands": result.Commands,
	})
}
