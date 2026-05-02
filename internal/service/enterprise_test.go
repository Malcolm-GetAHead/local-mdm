package service

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEnterpriseRepo struct {
	enterprises []*models.Enterprise
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	deleteErr   error
}

func (m *mockEnterpriseRepo) Create(_ context.Context, e *models.Enterprise) error {
	if m.createErr != nil {
		return m.createErr
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	m.enterprises = append(m.enterprises, e)
	return nil
}
func (m *mockEnterpriseRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Enterprise, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, e := range m.enterprises {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, fmt.Errorf("enterprise not found: %w", apperrors.ErrNotFound)
}
func (m *mockEnterpriseRepo) List(_ context.Context, limit, offset int) ([]*models.Enterprise, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.enterprises, len(m.enterprises), nil
}
func (m *mockEnterpriseRepo) Update(_ context.Context, e *models.Enterprise) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}
func (m *mockEnterpriseRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func TestEnterpriseService_CRUD(t *testing.T) {
	logger := slog.Default()
	repo := &mockEnterpriseRepo{}
	svc := NewEnterpriseService(repo, logger)
	ctx := context.Background()

	// Create
	ent := &models.Enterprise{Name: "Test Corp", Slug: "test-corp"}
	require.NoError(t, svc.Create(ctx, ent))
	assert.NotEqual(t, uuid.Nil, ent.ID)

	// Get
	got, err := svc.Get(ctx, ent.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Corp", got.Name)

	// List
	list, total, err := svc.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)

	// Update
	ent.Name = "Updated Corp"
	require.NoError(t, svc.Update(ctx, ent))

	// Delete
	require.NoError(t, svc.Delete(ctx, ent.ID))
}

func TestEnterpriseService_GetNotFound(t *testing.T) {
	svc := NewEnterpriseService(&mockEnterpriseRepo{}, slog.Default())
	_, err := svc.Get(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrNotFound)
}

func TestEnterpriseService_CreateError(t *testing.T) {
	repo := &mockEnterpriseRepo{createErr: fmt.Errorf("db error")}
	svc := NewEnterpriseService(repo, slog.Default())
	err := svc.Create(context.Background(), &models.Enterprise{Name: "Test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create enterprise")
}
