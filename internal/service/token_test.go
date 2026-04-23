package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTokenRepo struct {
	tokens map[string]*models.APIToken // keyed by hash
	byID   map[uuid.UUID]*models.APIToken
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{tokens: map[string]*models.APIToken{}, byID: map[uuid.UUID]*models.APIToken{}}
}

func (m *mockTokenRepo) Create(_ context.Context, t *models.APIToken) error {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	m.tokens[t.TokenHash] = t
	m.byID[t.ID] = t
	return nil
}
func (m *mockTokenRepo) GetByHash(_ context.Context, hash string) (*models.APIToken, error) {
	if t, ok := m.tokens[hash]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("token not found")
}
func (m *mockTokenRepo) List(_ context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	var out []*models.APIToken
	for _, t := range m.byID {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m *mockTokenRepo) Revoke(_ context.Context, id uuid.UUID) error {
	if _, ok := m.byID[id]; !ok {
		return fmt.Errorf("token not found")
	}
	now := time.Now()
	m.byID[id].RevokedAt = &now
	return nil
}
func (m *mockTokenRepo) UpdateLastUsed(_ context.Context, id uuid.UUID) error {
	return nil
}

type mockUserRepoForToken struct {
	users map[uuid.UUID]*models.User
}

func (m *mockUserRepoForToken) Create(_ context.Context, u *models.User) error { return nil }
func (m *mockUserRepoForToken) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found")
}
func (m *mockUserRepoForToken) GetByEmail(_ context.Context, _ uuid.UUID, _ string) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockUserRepoForToken) List(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepoForToken) Update(_ context.Context, _ *models.User) error   { return nil }
func (m *mockUserRepoForToken) Deactivate(_ context.Context, _ uuid.UUID) error { return nil }

func TestTokenService_Create(t *testing.T) {
	userID := uuid.New()
	ur := &mockUserRepoForToken{users: map[uuid.UUID]*models.User{userID: {BaseModel: models.BaseModel{ID: userID}, IsActive: true}}}
	tr := newMockTokenRepo()
	svc := NewTokenService(tr, ur, testLogger())

	result, err := svc.Create(context.Background(), userID, "my-token", nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result.Plaintext, "lmdm_"))
	assert.NotEmpty(t, result.Token.TokenHash)
	assert.Equal(t, "my-token", result.Token.Name)
}

func TestTokenService_Create_EmptyName(t *testing.T) {
	svc := NewTokenService(newMockTokenRepo(), &mockUserRepoForToken{users: map[uuid.UUID]*models.User{}}, testLogger())
	_, err := svc.Create(context.Background(), uuid.New(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestTokenService_Create_UserNotFound(t *testing.T) {
	svc := NewTokenService(newMockTokenRepo(), &mockUserRepoForToken{users: map[uuid.UUID]*models.User{}}, testLogger())
	_, err := svc.Create(context.Background(), uuid.New(), "tok", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestTokenService_Validate(t *testing.T) {
	userID := uuid.New()
	user := &models.User{BaseModel: models.BaseModel{ID: userID}, IsActive: true, Email: "test@example.com"}
	ur := &mockUserRepoForToken{users: map[uuid.UUID]*models.User{userID: user}}
	tr := newMockTokenRepo()
	svc := NewTokenService(tr, ur, testLogger())

	result, err := svc.Create(context.Background(), userID, "tok", nil)
	require.NoError(t, err)

	u, tok, err := svc.Validate(context.Background(), result.Plaintext)
	require.NoError(t, err)
	assert.Equal(t, userID, u.ID)
	assert.Equal(t, "tok", tok.Name)
}

func TestTokenService_Validate_InactiveUser(t *testing.T) {
	userID := uuid.New()
	user := &models.User{BaseModel: models.BaseModel{ID: userID}, IsActive: false}
	ur := &mockUserRepoForToken{users: map[uuid.UUID]*models.User{userID: user}}
	tr := newMockTokenRepo()
	svc := NewTokenService(tr, ur, testLogger())

	result, err := svc.Create(context.Background(), userID, "tok", nil)
	require.NoError(t, err)

	_, _, err = svc.Validate(context.Background(), result.Plaintext)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deactivated")
}

func TestTokenService_ListAndRevoke(t *testing.T) {
	userID := uuid.New()
	ur := &mockUserRepoForToken{users: map[uuid.UUID]*models.User{userID: {BaseModel: models.BaseModel{ID: userID}, IsActive: true}}}
	tr := newMockTokenRepo()
	svc := NewTokenService(tr, ur, testLogger())

	_, err := svc.Create(context.Background(), userID, "tok1", nil)
	require.NoError(t, err)
	result2, err := svc.Create(context.Background(), userID, "tok2", nil)
	require.NoError(t, err)

	tokens, err := svc.List(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, tokens, 2)

	err = svc.Revoke(context.Background(), result2.Token.ID)
	require.NoError(t, err)
	assert.NotNil(t, tr.byID[result2.Token.ID].RevokedAt)
}
