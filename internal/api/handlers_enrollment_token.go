package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/service"
)

func (s *Server) handleCreateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnterpriseID string `json:"enterprise_id"`
		Description  string `json:"description"`
		MaxUses      *int   `json:"max_uses"`
		ExpiresIn    string `json:"expires_in"` // duration string e.g. "24h", "168h"
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	enterpriseID, err := uuid.Parse(req.EnterpriseID)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "valid enterprise_id is required")
		return
	}

	// Verify enterprise exists
	if _, err := s.enterpriseService.Get(r.Context(), enterpriseID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to verify enterprise", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to verify enterprise")
		return
	}

	// Parse expires_in duration from string
	var expiresIn time.Duration
	if req.ExpiresIn != "" {
		expiresIn, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "invalid expires_in duration")
			return
		}
	}

	// Resolve created_by from authenticated user (only if user exists in local DB)
	var createdBy *uuid.UUID
	if authUser, err := auth.UserFromContext(r.Context()); err == nil {
		if uid, parseErr := uuid.Parse(authUser.ID); parseErr == nil {
			if _, lookupErr := s.userService.Get(r.Context(), uid); lookupErr == nil {
				createdBy = &uid
			}
		}
	}

	token, err := s.enrollmentTokenService.CreateToken(r.Context(), service.CreateTokenRequest{
		EnterpriseID: enterpriseID,
		Description:  req.Description,
		MaxUses:      req.MaxUses,
		ExpiresIn:    expiresIn,
		CreatedBy:    createdBy,
	})
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		s.logger.Error("failed to create enrollment token", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create enrollment token")
		return
	}

	s.logAudit(r, "enrollment_token.create", "enrollment_token", token.ID, map[string]interface{}{
		"enterprise_id": enterpriseID,
		"max_uses":      req.MaxUses,
		"expires_at":    token.ExpiresAt,
	})

	// Return token with platform-specific enrollment instructions
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	macosURL := fmt.Sprintf("%s://%s/api/v1/macos/enroll/%s?token=%s", scheme, r.Host, enterpriseID, token.Token)

	respondJSON(w, r, http.StatusCreated, map[string]interface{}{
		"id":               token.ID,
		"enterprise_id":    token.EnterpriseID,
		"token":            token.Token,
		"email":            token.Token + "@localmdm.local",
		"macos_enroll_url": macosURL,
		"description":      token.Description,
		"max_uses":         token.MaxUses,
		"uses_remaining":   token.UsesRemaining,
		"status":           token.Status,
		"expires_at":       token.ExpiresAt,
		"created_at":       token.CreatedAt,
	})
}

func (s *Server) handleListEnrollmentTokens(w http.ResponseWriter, r *http.Request) {
	enterpriseIDStr := r.URL.Query().Get("enterprise_id")
	if enterpriseIDStr == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "enterprise_id query parameter is required")
		return
	}
	enterpriseID, err := uuid.Parse(enterpriseIDStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise_id format")
		return
	}

	limit, offset := parsePagination(r)
	tokens, total, err := s.enrollmentTokenService.List(r.Context(), enterpriseID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list enrollment tokens", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list enrollment tokens")
		return
	}

	respondPaginated(w, r, http.StatusOK, tokens, total, limit, offset)
}

func (s *Server) handleRevokeEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid token ID format")
		return
	}

	if err := s.enrollmentTokenService.Revoke(r.Context(), id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enrollment token not found or already revoked")
			return
		}
		s.logger.Error("failed to revoke enrollment token", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to revoke enrollment token")
		return
	}

	s.logAudit(r, "enrollment_token.revoke", "enrollment_token", id, nil)

	w.WriteHeader(http.StatusNoContent)
}

// validateEnrollmentToken checks if a token is valid for enrollment.
// Returns the token if valid, or nil and an error message if not.
// If an active token is time-expired, updates its status to expired before rejecting.
func (s *Server) validateEnrollmentToken(r *http.Request, tokenStr string) (*models.EnrollmentToken, string) {
	return s.enrollmentTokenService.Validate(r.Context(), tokenStr)
}

// respondSOAPFault sends a SOAP fault response for enrollment errors.
func respondSOAPFault(w http.ResponseWriter, code, reason string) {
	fault := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://www.w3.org/2005/08/addressing">
  <s:Body>
    <s:Fault>
      <s:Code><s:Value>%s</s:Value></s:Code>
      <s:Reason><s:Text xml:lang="en">%s</s:Text></s:Reason>
    </s:Fault>
  </s:Body>
</s:Envelope>`, code, reason)
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fault)))
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(fault))
}
