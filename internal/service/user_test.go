package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	users map[uuid.UUID]*models.User
}

func newMockUserRepo() *mockUserRepo { return &mockUserRepo{users: map[uuid.UUID]*models.User{}} }

func (m *mockUserRepo) Create(_ context.Context, u *models.User) error {
	u.ID = uuid.New()
	m.users[u.ID] = u
	return nil
}
func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found")
}
func (m *mockUserRepo) GetByEmail(_ context.Context, eid uuid.UUID, email string) (*models.User, error) {
	for _, u := range m.users {
		if u.EnterpriseID == eid && u.Email == email {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}
func (m *mockUserRepo) List(_ context.Context, eid uuid.UUID, limit, offset int) ([]*models.User, int, error) {
	var out []*models.User
	for _, u := range m.users {
		if u.EnterpriseID == eid {
			out = append(out, u)
		}
	}
	return out, len(out), nil
}
func (m *mockUserRepo) Update(_ context.Context, u *models.User) error {
	m.users[u.ID] = u
	return nil
}
func (m *mockUserRepo) Deactivate(_ context.Context, id uuid.UUID) error {
	if u, ok := m.users[id]; ok {
		u.IsActive = false
		return nil
	}
	return fmt.Errorf("user not found")
}

func TestUserService_Create(t *testing.T) {
	svc := NewUserService(newMockUserRepo(), testLogger())
	u := &models.User{Email: "test@example.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	err := svc.Create(context.Background(), u)
	require.NoError(t, err)
	assert.True(t, u.IsActive)
	assert.NotEqual(t, uuid.Nil, u.ID)
}

func TestUserService_Create_EmptyEmail(t *testing.T) {
	svc := NewUserService(newMockUserRepo(), testLogger())
	err := svc.Create(context.Background(), &models.User{Role: models.RoleAdmin})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestUserService_Create_InvalidRole(t *testing.T) {
	svc := NewUserService(newMockUserRepo(), testLogger())
	err := svc.Create(context.Background(), &models.User{Email: "a@b.com", Role: "hacker"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestUserService_GetAndList(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	eid := uuid.New()
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: eid}
	require.NoError(t, svc.Create(context.Background(), u))

	got, err := svc.Get(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", got.Email)

	list, total, err := svc.List(context.Background(), eid, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)
}

func TestUserService_Update(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), u))

	newName := "Updated Name"
	newRole := models.RoleViewer
	updated, err := svc.Update(context.Background(), u.ID, &newName, &newRole, nil)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.FullName)
	assert.Equal(t, models.RoleViewer, updated.Role)
}

func TestUserService_Update_InvalidRole(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), u))

	badRole := "hacker"
	_, err := svc.Update(context.Background(), u.ID, nil, &badRole, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestUserService_Deactivate(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo, testLogger())
	u := &models.User{Email: "a@b.com", Role: models.RoleAdmin, EnterpriseID: uuid.New()}
	require.NoError(t, svc.Create(context.Background(), u))

	err := svc.Deactivate(context.Background(), u.ID)
	require.NoError(t, err)
	assert.False(t, repo.users[u.ID].IsActive)
}
