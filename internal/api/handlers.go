package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// Health check handler
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	timeout := s.config.Server.HealthCheckTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	type healthCheck struct {
		Status   string            `json:"status"`
		Version  string            `json:"version"`
		Checks   map[string]string `json:"checks"`
		Timestamp time.Time        `json:"timestamp"`
	}

	checks := make(map[string]string)
	allHealthy := true

	// Check database
	if err := s.db.Health(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}

	// Check Keycloak
	if err := s.authMiddleware.HealthCheck(ctx); err != nil {
		checks["keycloak"] = "degraded: " + err.Error()
	} else {
		checks["keycloak"] = "healthy"
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	health := healthCheck{
		Status:    status,
		Version:   "1.0.0",
		Checks:    checks,
		Timestamp: time.Now(),
	}

	respondJSON(w, r, httpStatus, health)
}

// Version handler
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, r, http.StatusOK, map[string]string{
		"version": "1.0.0",
		"build":   "dev",
	})
}

// Auth handlers
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	
	if err := req.Validate(); err != nil {
		respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	
	kc := auth.NewKeycloakClient(
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		s.config.Keycloak.ClientSecret,
	)
	
	tokenResp, err := kc.Login(req.Username, req.Password)
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "login_failed", "Invalid credentials")
		return
	}
	
	respondJSON(w, r, http.StatusOK, tokenResp)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	
	if req.RefreshToken == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "refresh_token is required")
		return
	}
	if len(req.RefreshToken) > 2048 {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "refresh_token too long")
		return
	}
	
	kc := auth.NewKeycloakClient(
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		s.config.Keycloak.ClientSecret,
	)
	
	tokenResp, err := kc.RefreshToken(req.RefreshToken)
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "refresh_failed", "Invalid refresh token")
		return
	}
	
	respondJSON(w, r, http.StatusOK, tokenResp)
}

// Enterprise handlers

func (s *Server) handleListEnterprises(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	enterprises, total, err := s.enterpriseRepo.List(r.Context(), limit, offset)
	if err != nil {
		s.logger.Error("failed to list enterprises", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list enterprises")
		return
	}

	respondPaginated(w, r, http.StatusOK, enterprises, total, limit, offset)
}

func (s *Server) handleCreateEnterprise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string        `json:"name"`
		Slug     string        `json:"slug"`
		Settings models.JSONB  `json:"settings"`
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

	if err := s.enterpriseRepo.Create(r.Context(), enterprise); err != nil {
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

	enterprise, err := s.enterpriseRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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

	enterprise, err := s.enterpriseRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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

	if err := s.enterpriseRepo.Update(r.Context(), enterprise); err != nil {
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

	if err := s.enterpriseRepo.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
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

// Device handlers

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)

	devices, total, err := s.deviceRepo.List(r.Context(), user.EnterpriseID, limit, offset)
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

	device, err := s.deviceRepo.GetByID(r.Context(), id)
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

	device, err := s.deviceRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to get device for lock", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to lock device")
		return
	}

	// Create command for platform dispatch
	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: models.CommandTypeLock,
	}
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create lock command", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to lock device")
		return
	}

	device.Status = models.DeviceStatusLost
	if err := s.deviceRepo.Update(r.Context(), device); err != nil {
		s.logger.Error("failed to lock device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to lock device")
		return
	}

	s.cmdDispatcher.Enqueue(device, cmd)

	s.logAudit(r, "device.lock", "device", id, map[string]interface{}{
		"platform":   device.Platform,
		"command_id": cmd.ID,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  device,
		"command": cmd,
	})
}

func (s *Server) handleWipeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.deviceRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to get device for wipe", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to wipe device")
		return
	}

	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: models.CommandTypeWipe,
	}
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create wipe command", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to wipe device")
		return
	}

	device.Status = models.DeviceStatusWiped
	if err := s.deviceRepo.Update(r.Context(), device); err != nil {
		s.logger.Error("failed to wipe device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to wipe device")
		return
	}

	s.cmdDispatcher.Enqueue(device, cmd)

	s.lifecycleService.OnWipe(r.Context(), device)

	s.logAudit(r, "device.wipe", "device", id, map[string]interface{}{
		"platform":   device.Platform,
		"command_id": cmd.ID,
	})

	s.logAudit(r, "device.lifecycle.wipe", "device", id, map[string]interface{}{
		"platform": device.Platform,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  device,
		"command": cmd,
	})
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.deviceRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to get device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	var req struct {
		Name         *string      `json:"name"`
		Model        *string      `json:"model"`
		OSVersion    *string      `json:"os_version"`
		Status       *string      `json:"status"`
		PlatformData *models.JSONB `json:"platform_data"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.Name != nil {
		device.Name = *req.Name
	}
	if req.Model != nil {
		device.Model = *req.Model
	}
	if req.OSVersion != nil {
		device.OSVersion = *req.OSVersion
	}
	if req.Status != nil {
		device.Status = *req.Status
	}
	if req.PlatformData != nil {
		device.PlatformData = *req.PlatformData
	}

	if err := s.deviceRepo.Update(r.Context(), device); err != nil {
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

	// Fetch device before deletion for lifecycle hooks
	device, err := s.deviceRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		s.logger.Error("failed to get device for delete", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete device")
		return
	}

	if err := s.deviceRepo.Delete(r.Context(), id); err != nil {
		s.logger.Error("failed to delete device", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete device")
		return
	}

	s.lifecycleService.OnDelete(r.Context(), device)

	s.logAudit(r, "device.delete", "device", id, nil)
	s.logAudit(r, "device.lifecycle.delete", "device", id, map[string]interface{}{
		"platform": device.Platform,
	})

	w.WriteHeader(http.StatusNoContent)
}

// Policy handlers

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)

	policies, total, err := s.policyRepo.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list policies", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list policies")
		return
	}

	respondPaginated(w, r, http.StatusOK, policies, total, limit, offset)
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req struct {
		Name         string       `json:"name"`
		Description  string       `json:"description"`
		Platform     string       `json:"platform"`
		PolicyType   string       `json:"policy_type"`
		PolicyConfig models.JSONB `json:"policy_config"`
		IsActive     bool         `json:"is_active"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	if !isValidPlatform(req.Platform) {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "platform must be windows, macos, or android")
		return
	}
	if !isValidPolicyType(req.PolicyType) {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "invalid policy_type")
		return
	}

	policy := &models.Policy{
		EnterpriseID: user.EnterpriseID,
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Platform:     req.Platform,
		PolicyType:   req.PolicyType,
		PolicyConfig: req.PolicyConfig,
		IsActive:     req.IsActive,
	}

	if err := s.policyRepo.Create(r.Context(), policy); err != nil {
		s.logger.Error("failed to create policy", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create policy")
		return
	}

	s.logAudit(r, "policy.create", "policy", policy.ID, map[string]interface{}{
		"name":     policy.Name,
		"platform": policy.Platform,
	})

	respondJSON(w, r, http.StatusCreated, policy)
}

func (s *Server) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	policy, err := s.policyRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Policy not found")
			return
		}
		s.logger.Error("failed to get policy", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get policy")
		return
	}

	respondJSON(w, r, http.StatusOK, policy)
}

func (s *Server) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	policy, err := s.policyRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Policy not found")
			return
		}
		s.logger.Error("failed to get policy", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get policy")
		return
	}

	var req struct {
		Name         *string      `json:"name"`
		Description  *string      `json:"description"`
		PolicyConfig *models.JSONB `json:"policy_config"`
		IsActive     *bool        `json:"is_active"`
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
		policy.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		policy.Description = strings.TrimSpace(*req.Description)
	}
	if req.PolicyConfig != nil {
		policy.PolicyConfig = *req.PolicyConfig
	}
	if req.IsActive != nil {
		policy.IsActive = *req.IsActive
	}

	if err := s.policyRepo.Update(r.Context(), policy); err != nil {
		s.logger.Error("failed to update policy", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update policy")
		return
	}

	s.logAudit(r, "policy.update", "policy", id, map[string]interface{}{
		"name": policy.Name,
	})

	respondJSON(w, r, http.StatusOK, policy)
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	if err := s.policyRepo.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Policy not found")
			return
		}
		s.logger.Error("failed to delete policy", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete policy")
		return
	}

	s.logAudit(r, "policy.delete", "policy", id, nil)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignPolicy(w http.ResponseWriter, r *http.Request) {
	policyID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	var req struct {
		DeviceIDs []uuid.UUID `json:"device_ids"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if len(req.DeviceIDs) == 0 {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "device_ids is required")
		return
	}

	// Verify policy exists
	if _, err := s.policyRepo.GetByID(r.Context(), policyID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Policy not found")
			return
		}
		s.logger.Error("failed to get policy", "error", err, "id", policyID)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to assign policy")
		return
	}

	for _, deviceID := range req.DeviceIDs {
		if err := s.policyRepo.AssignToDevice(r.Context(), deviceID, policyID); err != nil {
			s.logger.Error("failed to assign policy", "error", err, "policy_id", policyID, "device_id", deviceID)
			respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to assign policy")
			return
		}
	}

	s.logAudit(r, "policy.assign", "policy", policyID, map[string]interface{}{
		"device_ids": req.DeviceIDs,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"policy_id":  policyID,
		"device_ids": req.DeviceIDs,
		"status":     "assigned",
	})
}

func (s *Server) handleUnassignPolicy(w http.ResponseWriter, r *http.Request) {
	policyID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	deviceID, err := parseUUIDParam(r, "device_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	if err := s.policyRepo.UnassignFromDevice(r.Context(), deviceID, policyID); err != nil {
		s.logger.Error("failed to unassign policy", "error", err, "policy_id", policyID, "device_id", deviceID)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to unassign policy")
		return
	}

	s.logAudit(r, "policy.unassign", "policy", policyID, map[string]interface{}{
		"device_id": deviceID,
	})

	w.WriteHeader(http.StatusNoContent)
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

	certs, total, err := s.certRepo.List(r.Context(), deviceID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list certificates", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list certificates")
		return
	}

	respondPaginated(w, r, http.StatusOK, certs, total, limit, offset)
}

// Audit log handlers
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)

	logs, total, err := s.auditLogRepo.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list audit logs", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list audit logs")
		return
	}

	respondPaginated(w, r, http.StatusOK, logs, total, limit, offset)
}

// Helper functions

func parseJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(mux.Vars(r)[name])
}

func parsePagination(r *http.Request) (int, int) {
	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func respondPaginated(w http.ResponseWriter, r *http.Request, status int, data interface{}, total, limit, offset int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	requestID, _ := r.Context().Value(requestIDKey).(string)
	page := (offset / limit) + 1
	totalPages := (total + limit - 1) / limit

	response := Response{
		Data: data,
		Meta: &MetaInfo{
			Timestamp:  time.Now(),
			RequestID:  requestID,
			Page:       page,
			PerPage:    limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) logAudit(r *http.Request, action, resourceType string, resourceID uuid.UUID, details map[string]interface{}) {
	var enterpriseID, userID uuid.UUID
	if user, err := auth.UserFromContext(r.Context()); err == nil {
		enterpriseID = user.EnterpriseID
		if uid, err := uuid.Parse(user.ID); err == nil {
			userID = uid
		}
	}

	// Propagate request ID into audit details
	if details == nil {
		details = make(map[string]interface{})
	}
	if reqID, ok := r.Context().Value(requestIDKey).(string); ok && reqID != "" {
		details["request_id"] = reqID
	}

	ip := net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])

	_ = s.auditLogger.Log(r.Context(), audit.Event{
		EnterpriseID: enterpriseID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    ip,
		UserAgent:    r.UserAgent(),
	})
}

func isValidPlatform(p string) bool {
	return p == models.PlatformWindows || p == models.PlatformMacOS || p == models.PlatformAndroid || p == models.PlatformAll
}

func isValidPolicyType(t string) bool {
	switch t {
	case models.PolicyTypeWiFi, models.PolicyTypeVPN, models.PolicyTypeSecurity,
		models.PolicyTypeApp, models.PolicyTypeRestriction, models.PolicyTypeCompliance:
		return true
	}
	return false
}

func isDuplicateError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "already exists")
}

// --- Sprint 3: App Management Handlers ---

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)
	apps, total, err := s.appRepo.List(r.Context(), user.EnterpriseID, limit, offset)
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
		Name        string      `json:"name"`
		Platform    string      `json:"platform"`
		Identifier  string      `json:"identifier"`
		Version     string      `json:"version"`
		InstallType string      `json:"install_type"`
		AppConfig   models.JSONB `json:"app_config"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Name == "" || req.Platform == "" || req.Identifier == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "name, platform, and identifier are required")
		return
	}
	if !isValidPlatform(req.Platform) {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "Invalid platform")
		return
	}
	if req.InstallType == "" {
		req.InstallType = models.AppInstallRequired
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

	if err := s.appRepo.Create(r.Context(), app); err != nil {
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

	app, err := s.appRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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

	app, err := s.appRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	var req struct {
		Name        *string      `json:"name"`
		Version     *string      `json:"version"`
		InstallType *string      `json:"install_type"`
		AppConfig   *models.JSONB `json:"app_config"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Name != nil {
		app.Name = *req.Name
	}
	if req.Version != nil {
		app.Version = *req.Version
	}
	if req.InstallType != nil {
		app.InstallType = *req.InstallType
	}
	if req.AppConfig != nil {
		app.AppConfig = *req.AppConfig
	}

	if err := s.appRepo.Update(r.Context(), app); err != nil {
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

	if err := s.appRepo.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
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

	app, err := s.appRepo.GetByID(r.Context(), appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "App not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	var req struct {
		DeviceIDs []uuid.UUID `json:"device_ids"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if len(req.DeviceIDs) == 0 {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "device_ids is required")
		return
	}

	var commands []*models.DeviceCommand
	for _, deviceID := range req.DeviceIDs {
		cmd := &models.DeviceCommand{
			DeviceID:    deviceID,
			CommandType: models.CommandTypeInstallApp,
			CommandData: models.JSONB{
				"app_id":     app.ID,
				"identifier": app.Identifier,
				"name":       app.Name,
				"platform":   app.Platform,
			},
		}
		if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
			s.logger.Error("failed to create deploy command", "error", err, "device_id", deviceID)
			continue
		}
		// Dispatch to platform async
		if device, err := s.deviceRepo.GetByID(r.Context(), deviceID); err == nil {
			s.cmdDispatcher.Enqueue(device, cmd)
		}
		commands = append(commands, cmd)
	}

	s.logAudit(r, "app.deploy", "app", appID, map[string]interface{}{
		"device_ids": req.DeviceIDs,
		"commands":   len(commands),
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"app":      app,
		"commands": commands,
	})
}

// --- Sprint 3: Command & Profile Handlers ---

func (s *Server) handleSendCommand(w http.ResponseWriter, r *http.Request) {
	deviceID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
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

	var req struct {
		CommandType string      `json:"command_type"`
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
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
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
	cmds, total, err := s.cmdRepo.ListByDevice(r.Context(), deviceID, limit, offset)
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

	device, err := s.deviceRepo.GetByID(r.Context(), deviceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	var req struct {
		ProfileType string      `json:"profile_type"`
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
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
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

func (s *Server) handleRestartDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
		return
	}

	device, err := s.deviceRepo.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get device")
		return
	}

	// Restart is supported on macOS and Android only
	if device.Platform == models.PlatformWindows {
		respondError(w, r, http.StatusBadRequest, "unsupported", "Restart is not supported on Windows devices")
		return
	}

	cmd := &models.DeviceCommand{
		DeviceID:    device.ID,
		CommandType: models.CommandTypeRestart,
	}
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
		s.logger.Error("failed to create restart command", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to restart device")
		return
	}

	s.cmdDispatcher.Enqueue(device, cmd)

	s.logAudit(r, "device.restart", "device", id, map[string]interface{}{
		"platform":   device.Platform,
		"command_id": cmd.ID,
	})

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"device":  device,
		"command": cmd,
	})
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

	device, err := s.deviceRepo.GetByID(r.Context(), deviceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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
	if err := s.cmdRepo.Create(r.Context(), cmd); err != nil {
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
