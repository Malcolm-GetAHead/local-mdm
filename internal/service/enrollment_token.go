package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// EnrollmentTokenRepository is the interface for enrollment token data access.
type EnrollmentTokenRepository interface {
	Create(ctx context.Context, token *models.EnrollmentToken) error
	GetByToken(ctx context.Context, token string) (*models.EnrollmentToken, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.EnrollmentToken, int, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	DecrementUses(ctx context.Context, id uuid.UUID) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	ExpireTokens(ctx context.Context) (int64, error)
}

// EnrollmentTokenService handles enrollment token business logic.
type EnrollmentTokenService struct {
	repo   EnrollmentTokenRepository
	logger *slog.Logger
}

// NewEnrollmentTokenService creates a new enrollment token service.
func NewEnrollmentTokenService(repo EnrollmentTokenRepository, logger *slog.Logger) *EnrollmentTokenService {
	return &EnrollmentTokenService{repo: repo, logger: logger}
}

// generateToken produces a cryptographically random 32-character hex token.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateTokenRequest is the input DTO for creating an enrollment token.
type CreateTokenRequest struct {
	EnterpriseID uuid.UUID
	Description  string
	MaxUses      *int
	ExpiresIn    time.Duration // 0 = default 24h
	CreatedBy    *uuid.UUID
}

// CreateToken validates input, generates a token string, and persists the enrollment token.
func (s *EnrollmentTokenService) CreateToken(ctx context.Context, req CreateTokenRequest) (*models.EnrollmentToken, error) {
	if req.ExpiresIn == 0 {
		req.ExpiresIn = 24 * time.Hour
	}
	if req.ExpiresIn < time.Minute {
		return nil, fmt.Errorf("expires_in must be at least 1 minute: %w", apperrors.ErrValidation)
	}
	if req.MaxUses != nil && *req.MaxUses < 1 {
		return nil, fmt.Errorf("max_uses must be at least 1: %w", apperrors.ErrValidation)
	}

	tokenStr, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	token := &models.EnrollmentToken{
		EnterpriseID:  req.EnterpriseID,
		Token:         tokenStr,
		Description:   strings.TrimSpace(req.Description),
		MaxUses:       req.MaxUses,
		UsesRemaining: req.MaxUses,
		ExpiresAt:     time.Now().Add(req.ExpiresIn),
		CreatedBy:     req.CreatedBy,
	}
	if err := s.repo.Create(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to create enrollment token: %w", err)
	}
	return token, nil
}

// Create creates a new enrollment token.
func (s *EnrollmentTokenService) Create(ctx context.Context, token *models.EnrollmentToken) error {
	if err := s.repo.Create(ctx, token); err != nil {
		return fmt.Errorf("failed to create enrollment token: %w", err)
	}
	return nil
}

// GetByToken retrieves an enrollment token by its token string.
func (s *EnrollmentTokenService) GetByToken(ctx context.Context, token string) (*models.EnrollmentToken, error) {
	return s.repo.GetByToken(ctx, token)
}

// List returns enrollment tokens for an enterprise.
func (s *EnrollmentTokenService) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.EnrollmentToken, int, error) {
	return s.repo.List(ctx, enterpriseID, limit, offset)
}

// Revoke revokes an enrollment token.
func (s *EnrollmentTokenService) Revoke(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Revoke(ctx, id); err != nil {
		return fmt.Errorf("failed to revoke enrollment token: %w", err)
	}
	return nil
}

// DecrementUses decrements the remaining uses of a token.
func (s *EnrollmentTokenService) DecrementUses(ctx context.Context, id uuid.UUID) error {
	return s.repo.DecrementUses(ctx, id)
}

// SetStatus sets the status of an enrollment token.
func (s *EnrollmentTokenService) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.repo.SetStatus(ctx, id, status)
}

// ExpireTokens marks active tokens past their expiry as expired.
func (s *EnrollmentTokenService) ExpireTokens(ctx context.Context) (int64, error) {
	return s.repo.ExpireTokens(ctx)
}

// Validate checks if a token is valid for enrollment.
// Returns the token if valid, or nil and an error message if not.
// If an active token is time-expired, updates its status to expired before rejecting.
func (s *EnrollmentTokenService) Validate(ctx context.Context, tokenStr string) (*models.EnrollmentToken, string) {
	token, err := s.repo.GetByToken(ctx, tokenStr)
	if err != nil {
		return nil, ""
	}
	if token.Status == models.EnrollmentTokenStatusRevoked {
		return nil, "Enrollment token has been revoked"
	}
	if token.Status == models.EnrollmentTokenStatusExpired {
		return nil, "Enrollment token has expired"
	}
	// Active token but time-expired — update status before rejecting
	if time.Now().After(token.ExpiresAt) {
		_ = s.repo.SetStatus(ctx, token.ID, models.EnrollmentTokenStatusExpired)
		return nil, "Enrollment token has expired"
	}
	if token.UsesRemaining != nil && *token.UsesRemaining <= 0 {
		return nil, "Enrollment token has no remaining uses"
	}
	return token, ""
}
