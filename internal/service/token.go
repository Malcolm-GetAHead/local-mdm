package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

type TokenRepository interface {
	Create(ctx context.Context, token *models.APIToken) error
	GetByHash(ctx context.Context, tokenHash string) (*models.APIToken, error)
	List(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

type TokenService struct {
	tokenRepo TokenRepository
	userRepo  UserRepository
	logger    *slog.Logger
}

func NewTokenService(tokenRepo TokenRepository, userRepo UserRepository, logger *slog.Logger) *TokenService {
	return &TokenService{tokenRepo: tokenRepo, userRepo: userRepo, logger: logger}
}

const tokenPrefix = "lmdm_"

// TokenCreateResult holds the token metadata and the plaintext (returned once).
type TokenCreateResult struct {
	Token     *models.APIToken
	Plaintext string // returned once at creation, never stored
}

// Create generates a new API token for a user.
func (s *TokenService) Create(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (*TokenCreateResult, error) {
	if name == "" {
		return nil, fmt.Errorf("token name is required")
	}
	// Verify user exists
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return nil, fmt.Errorf("user not found: %w", apperrors.ErrNotFound)
	}

	// Generate random token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	plaintext := tokenPrefix + base64.URLEncoding.EncodeToString(raw)

	// Hash for storage
	hash := hashToken(plaintext)

	token := &models.APIToken{
		UserID:    userID,
		Name:      name,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	}
	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, err
	}
	return &TokenCreateResult{Token: token, Plaintext: plaintext}, nil
}

// Validate checks a plaintext token and returns the associated user.
func (s *TokenService) Validate(ctx context.Context, plaintext string) (*models.User, *models.APIToken, error) {
	hash := hashToken(plaintext)
	token, err := s.tokenRepo.GetByHash(ctx, hash)
	if err != nil {
		return nil, nil, err
	}
	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("token user not found: %w", apperrors.ErrNotFound)
	}
	if !user.IsActive {
		return nil, nil, fmt.Errorf("user is deactivated")
	}
	// Update last_used_at asynchronously (best-effort)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.tokenRepo.UpdateLastUsed(ctx, token.ID); err != nil {
			s.logger.Warn("failed to update token last_used_at", "token_id", token.ID, "error", err)
		}
	}()
	return user, token, nil
}

// List returns all tokens for a user (metadata only, no hashes).
func (s *TokenService) List(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	return s.tokenRepo.List(ctx, userID)
}

// Revoke marks a token as revoked.
func (s *TokenService) Revoke(ctx context.Context, id uuid.UUID) error {
	return s.tokenRepo.Revoke(ctx, id)
}

func hashToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return base64.URLEncoding.EncodeToString(h[:])
}
