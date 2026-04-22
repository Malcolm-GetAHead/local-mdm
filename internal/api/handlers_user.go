package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// User Management handlers (S5-11)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	limit, offset := parsePagination(r)
	users, total, err := s.userService.List(r.Context(), user.EnterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list users", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list users")
		return
	}
	respondPaginated(w, r, http.StatusOK, users, total, limit, offset)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	caller, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	u := &models.User{
		EnterpriseID: caller.EnterpriseID,
		Email:        req.Email,
		FullName:     req.FullName,
		Role:         req.Role,
	}
	if err := s.userService.Create(r.Context(), u); err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid role") {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		s.logger.Error("failed to create user", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create user")
		return
	}
	s.logAudit(r, "user.create", "user", u.ID, map[string]interface{}{"email": u.Email, "role": u.Role})
	respondJSON(w, r, http.StatusCreated, u)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid user ID format")
		return
	}
	u, err := s.userService.Get(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "User not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get user")
		return
	}
	respondJSON(w, r, http.StatusOK, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid user ID format")
		return
	}
	var req struct {
		FullName *string `json:"full_name"`
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	u, err := s.userService.Update(r.Context(), id, req.FullName, req.Role, req.IsActive)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "User not found")
			return
		}
		if strings.Contains(err.Error(), "invalid role") {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update user")
		return
	}
	s.logAudit(r, "user.update", "user", id, nil)
	respondJSON(w, r, http.StatusOK, u)
}

func (s *Server) handleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid user ID format")
		return
	}
	if err := s.userService.Deactivate(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "User not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to deactivate user")
		return
	}
	s.logAudit(r, "user.deactivate", "user", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// API Token handlers (S5-11)

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	caller, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	callerID, err := uuid.Parse(caller.ID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Invalid caller ID")
		return
	}
	result, err := s.tokenService.Create(r.Context(), callerID, req.Name, nil)
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		s.logger.Error("failed to create token", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create token")
		return
	}
	s.logAudit(r, "token.create", "api_token", result.Token.ID, map[string]interface{}{"name": req.Name})
	respondJSON(w, r, http.StatusCreated, map[string]interface{}{
		"token":     result.Token,
		"plaintext": result.Plaintext,
	})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	caller, err := auth.UserFromContext(r.Context())
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	callerID, _ := uuid.Parse(caller.ID)
	tokens, err := s.tokenService.List(r.Context(), callerID)
	if err != nil {
		s.logger.Error("failed to list tokens", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list tokens")
		return
	}
	respondJSON(w, r, http.StatusOK, tokens)
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid token ID format")
		return
	}
	if err := s.tokenService.Revoke(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Token not found")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to revoke token")
		return
	}
	s.logAudit(r, "token.revoke", "api_token", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
