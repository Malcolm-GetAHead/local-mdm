package service

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEnrollmentTokenRepo struct {
	tokens []*models.EnrollmentToken
}

func (m *mockEnrollmentTokenRepo) Create(_ context.Context, t *models.EnrollmentToken) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Status == "" {
		t.Status = models.EnrollmentTokenStatusActive
	}
	t.CreatedAt = time.Now()
	m.tokens = append(m.tokens, t)
	return nil
}
func (m *mockEnrollmentTokenRepo) GetByToken(_ context.Context, token string) (*models.EnrollmentToken, error) {
	for _, t := range m.tokens {
		if t.Token == token {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found: %w", apperrors.ErrNotFound)
}
func (m *mockEnrollmentTokenRepo) List(_ context.Context, eid uuid.UUID, limit, offset int) ([]*models.EnrollmentToken, int, error) {
	var result []*models.EnrollmentToken
	for _, t := range m.tokens {
		if t.EnterpriseID == eid {
			result = append(result, t)
		}
	}
	return result, len(result), nil
}
func (m *mockEnrollmentTokenRepo) Revoke(_ context.Context, id uuid.UUID) error {
	for _, t := range m.tokens {
		if t.ID == id {
			t.Status = models.EnrollmentTokenStatusRevoked
			return nil
		}
	}
	return fmt.Errorf("not found: %w", apperrors.ErrNotFound)
}
func (m *mockEnrollmentTokenRepo) DecrementUses(_ context.Context, id uuid.UUID) error {
	for _, t := range m.tokens {
		if t.ID == id && t.UsesRemaining != nil && *t.UsesRemaining > 0 {
			v := *t.UsesRemaining - 1
			t.UsesRemaining = &v
			return nil
		}
	}
	return nil
}
func (m *mockEnrollmentTokenRepo) SetStatus(_ context.Context, id uuid.UUID, status string) error {
	for _, t := range m.tokens {
		if t.ID == id {
			t.Status = status
			return nil
		}
	}
	return nil
}
func (m *mockEnrollmentTokenRepo) ExpireTokens(_ context.Context) (int64, error) { return 0, nil }

func TestEnrollmentTokenService_CreateToken(t *testing.T) {
	repo := &mockEnrollmentTokenRepo{}
	svc := NewEnrollmentTokenService(repo, slog.Default())
	ctx := context.Background()
	eid := uuid.New()

	// Happy path — default expiry
	maxUses := 5
	tok, err := svc.CreateToken(ctx, CreateTokenRequest{
		EnterpriseID: eid,
		Description:  "  test token  ",
		MaxUses:      &maxUses,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tok.ID)
	assert.Equal(t, eid, tok.EnterpriseID)
	assert.Equal(t, "test token", tok.Description) // trimmed
	assert.Len(t, tok.Token, 32)                   // 16 bytes hex-encoded
	assert.Equal(t, &maxUses, tok.MaxUses)
	assert.Equal(t, &maxUses, tok.UsesRemaining)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), tok.ExpiresAt, 5*time.Second)

	// Custom expiry
	tok2, err := svc.CreateToken(ctx, CreateTokenRequest{
		EnterpriseID: eid,
		ExpiresIn:    2 * time.Hour,
	})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), tok2.ExpiresAt, 5*time.Second)

	// Validation: expires_in too short
	_, err = svc.CreateToken(ctx, CreateTokenRequest{
		EnterpriseID: eid,
		ExpiresIn:    30 * time.Second,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrValidation)
	assert.Contains(t, err.Error(), "at least 1 minute")

	// Validation: max_uses < 1
	zero := 0
	_, err = svc.CreateToken(ctx, CreateTokenRequest{
		EnterpriseID: eid,
		MaxUses:      &zero,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrValidation)
	assert.Contains(t, err.Error(), "at least 1")

	// Nil max_uses = unlimited (no error)
	tok3, err := svc.CreateToken(ctx, CreateTokenRequest{
		EnterpriseID: eid,
	})
	require.NoError(t, err)
	assert.Nil(t, tok3.MaxUses)
	assert.Nil(t, tok3.UsesRemaining)
}

func TestEnrollmentTokenService_CRUD(t *testing.T) {
	repo := &mockEnrollmentTokenRepo{}
	svc := NewEnrollmentTokenService(repo, slog.Default())
	ctx := context.Background()
	eid := uuid.New()

	maxUses := 5
	tok := &models.EnrollmentToken{
		EnterpriseID:  eid,
		Token:         "abc123",
		Description:   "test",
		MaxUses:       &maxUses,
		UsesRemaining: &maxUses,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	// Seed directly via repo (CreateToken is tested separately)
	require.NoError(t, repo.Create(ctx, tok))
	assert.NotEqual(t, uuid.Nil, tok.ID)

	got, err := svc.GetByToken(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, tok.ID, got.ID)

	list, total, err := svc.List(ctx, eid, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)

	require.NoError(t, svc.Revoke(ctx, tok.ID))
}

func TestEnrollmentTokenService_Validate(t *testing.T) {
	repo := &mockEnrollmentTokenRepo{}
	svc := NewEnrollmentTokenService(repo, slog.Default())
	ctx := context.Background()

	maxUses := 1
	tok := &models.EnrollmentToken{
		EnterpriseID:  uuid.New(),
		Token:         "valid-token",
		MaxUses:       &maxUses,
		UsesRemaining: &maxUses,
		ExpiresAt:     time.Now().Add(time.Hour),
		Status:        models.EnrollmentTokenStatusActive,
	}
	repo.Create(ctx, tok)

	// Valid token
	got, msg := svc.Validate(ctx, "valid-token")
	assert.NotNil(t, got)
	assert.Empty(t, msg)

	// Not found
	got, msg = svc.Validate(ctx, "nonexistent")
	assert.Nil(t, got)

	// Revoked
	tok.Status = models.EnrollmentTokenStatusRevoked
	got, msg = svc.Validate(ctx, "valid-token")
	assert.Nil(t, got)
	assert.Contains(t, msg, "revoked")

	// Expired
	tok.Status = models.EnrollmentTokenStatusActive
	tok.ExpiresAt = time.Now().Add(-time.Hour)
	got, msg = svc.Validate(ctx, "valid-token")
	assert.Nil(t, got)
	assert.Contains(t, msg, "expired")

	// Exhausted
	tok.Status = models.EnrollmentTokenStatusActive
	tok.ExpiresAt = time.Now().Add(time.Hour)
	zero := 0
	tok.UsesRemaining = &zero
	got, msg = svc.Validate(ctx, "valid-token")
	assert.Nil(t, got)
	assert.Contains(t, msg, "no remaining uses")
}
