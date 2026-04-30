package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

func generateEnrollmentToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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
	if _, err := s.enterpriseRepo.GetByID(r.Context(), enterpriseID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to verify enterprise", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to verify enterprise")
		return
	}

	expiresIn := 24 * time.Hour // default 24h
	if req.ExpiresIn != "" {
		expiresIn, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "invalid expires_in duration")
			return
		}
		if expiresIn < time.Minute {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "expires_in must be at least 1 minute")
			return
		}
	}

	if req.MaxUses != nil && *req.MaxUses < 1 {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "max_uses must be at least 1")
		return
	}

	tokenStr, err := generateEnrollmentToken()
	if err != nil {
		s.logger.Error("failed to generate enrollment token", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate token")
		return
	}

	token := &models.EnrollmentToken{
		EnterpriseID:  enterpriseID,
		Token:         tokenStr,
		Description:   strings.TrimSpace(req.Description),
		MaxUses:       req.MaxUses,
		UsesRemaining: req.MaxUses,
		ExpiresAt:     time.Now().Add(expiresIn),
	}

	if err := s.enrollmentTokenRepo.Create(r.Context(), token); err != nil {
		s.logger.Error("failed to create enrollment token", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create enrollment token")
		return
	}

	s.logAudit(r, "enrollment_token.create", "enrollment_token", token.ID, map[string]interface{}{
		"enterprise_id": enterpriseID,
		"max_uses":      req.MaxUses,
		"expires_at":    token.ExpiresAt,
	})

	// Return token with the enrollment email address
	respondJSON(w, r, http.StatusCreated, map[string]interface{}{
		"id":             token.ID,
		"enterprise_id":  token.EnterpriseID,
		"token":          token.Token,
		"email":          token.Token + "@localmdm.local",
		"description":    token.Description,
		"max_uses":       token.MaxUses,
		"uses_remaining": token.UsesRemaining,
		"expires_at":     token.ExpiresAt,
		"created_at":     token.CreatedAt,
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
	tokens, total, err := s.enrollmentTokenRepo.List(r.Context(), enterpriseID, limit, offset)
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

	if err := s.enrollmentTokenRepo.Revoke(r.Context(), id); err != nil {
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
func (s *Server) validateEnrollmentToken(r *http.Request, tokenStr string) (*models.EnrollmentToken, string) {
	token, err := s.enrollmentTokenRepo.GetByToken(r.Context(), tokenStr)
	if err != nil {
		return nil, "Invalid enrollment token"
	}
	if token.RevokedAt != nil {
		return nil, "Enrollment token has been revoked"
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, "Enrollment token has expired"
	}
	if token.UsesRemaining != nil && *token.UsesRemaining <= 0 {
		return nil, "Enrollment token has no remaining uses"
	}
	return token, ""
}
