package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

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

	if err := s.policyService.Create(r.Context(), policy, user.ID); err != nil {
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
		Name         *string       `json:"name"`
		Description  *string       `json:"description"`
		PolicyConfig *models.JSONB `json:"policy_config"`
		IsActive     *bool         `json:"is_active"`
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

	user, _ := auth.UserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	if err := s.policyService.Update(r.Context(), policy, userID); err != nil {
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

// Policy versioning handlers (Sprint 4)

func (s *Server) handleListPolicyVersions(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	limit, offset := parsePagination(r)
	versions, total, err := s.policyService.ListVersions(r.Context(), id, limit, offset)
	if err != nil {
		s.logger.Error("failed to list policy versions", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list versions")
		return
	}

	respondPaginated(w, r, http.StatusOK, versions, total, limit, offset)
}

func (s *Server) handleRollbackPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID format")
		return
	}

	var req struct {
		Version int `json:"version"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Version < 1 {
		respondError(w, r, http.StatusBadRequest, "invalid_version", "Version must be >= 1")
		return
	}

	user, _ := auth.UserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	policy, err := s.policyService.Rollback(r.Context(), id, req.Version, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", err.Error())
			return
		}
		s.logger.Error("failed to rollback policy", "error", err, "id", id)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to rollback")
		return
	}

	s.logAudit(r, "policy.rollback", "policy", id, map[string]interface{}{"version": req.Version})
	respondJSON(w, r, http.StatusOK, policy)
}

func (s *Server) handleTranslatePolicy(w http.ResponseWriter, r *http.Request) {
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
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get policy")
		return
	}

	platform := r.URL.Query().Get("platform")
	if platform != "" {
		result, err := s.policyService.Translate(policy, platform)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "translation_error", err.Error())
			return
		}
		respondJSON(w, r, http.StatusOK, result)
		return
	}

	results := s.policyService.TranslateAll(policy)
	respondJSON(w, r, http.StatusOK, results)
}

func (s *Server) handleListPolicyTemplates(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, offset := parsePagination(r)
	policies, _, err := s.policyRepo.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list templates")
		return
	}

	// Filter to templates only
	var templates []*models.Policy
	for _, p := range policies {
		if p.IsTemplate {
			templates = append(templates, p)
		}
	}

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (s *Server) handleClonePolicyTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid template ID format")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Name == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "Name is required")
		return
	}

	user, _ := auth.UserFromContext(r.Context())
	enterpriseID := uuid.Nil
	userID := ""
	if user != nil {
		enterpriseID = user.EnterpriseID
		userID = user.ID
	}

	policy, err := s.policyService.CloneTemplate(r.Context(), templateID, enterpriseID, req.Name, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not a template") {
			respondError(w, r, http.StatusBadRequest, "invalid_template", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to clone template")
		return
	}

	s.logAudit(r, "policy.clone_template", "policy", policy.ID, map[string]interface{}{
		"template_id": templateID,
	})
	respondJSON(w, r, http.StatusCreated, policy)
}

// Policy Assignment handlers (Sprint 4)

func (s *Server) handleAssignPolicyToTarget(w http.ResponseWriter, r *http.Request) {
	policyID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID")
		return
	}
	var req struct {
		TargetType string    `json:"target_type"`
		TargetID   uuid.UUID `json:"target_id"`
		Priority   int       `json:"priority"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	assignment, err := s.groupService.AssignPolicy(r.Context(), policyID, req.TargetType, req.TargetID, req.Priority)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "assignment_error", err.Error())
		return
	}

	s.logAudit(r, "policy.assign_target", "policy", policyID, map[string]interface{}{
		"target_type": req.TargetType, "target_id": req.TargetID,
	})
	respondJSON(w, r, http.StatusCreated, assignment)
}

func (s *Server) handleUnassignPolicyFromTarget(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parseUUIDParam(r, "assignment_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid assignment ID")
		return
	}
	if err := s.groupService.UnassignPolicy(r.Context(), assignmentID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Assignment not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to unassign")
		return
	}
	s.logAudit(r, "policy.unassign_target", "policy_assignment", assignmentID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListPolicyAssignments(w http.ResponseWriter, r *http.Request) {
	policyID, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid policy ID")
		return
	}
	assignments, err := s.groupService.ListAssignmentsByPolicy(r.Context(), policyID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list assignments")
		return
	}
	respondJSON(w, r, http.StatusOK, assignments)
}
