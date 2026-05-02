package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// EnterpriseRepository is the interface for enterprise data access.
type EnterpriseRepository interface {
	Create(ctx context.Context, enterprise *models.Enterprise) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error)
	List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error)
	Update(ctx context.Context, enterprise *models.Enterprise) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// EnterpriseService handles enterprise business logic.
type EnterpriseService struct {
	repo   EnterpriseRepository
	logger *slog.Logger
}

// NewEnterpriseService creates a new enterprise service.
func NewEnterpriseService(repo EnterpriseRepository, logger *slog.Logger) *EnterpriseService {
	return &EnterpriseService{repo: repo, logger: logger}
}

// Create creates a new enterprise.
func (s *EnterpriseService) Create(ctx context.Context, enterprise *models.Enterprise) error {
	if err := s.repo.Create(ctx, enterprise); err != nil {
		return fmt.Errorf("failed to create enterprise: %w", err)
	}
	return nil
}

// Get retrieves an enterprise by ID.
func (s *EnterpriseService) Get(ctx context.Context, id uuid.UUID) (*models.Enterprise, error) {
	enterprise, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get enterprise %s: %w", id, err)
	}
	return enterprise, nil
}

// List returns a paginated list of enterprises.
func (s *EnterpriseService) List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error) {
	return s.repo.List(ctx, limit, offset)
}

// Update updates an enterprise.
func (s *EnterpriseService) Update(ctx context.Context, enterprise *models.Enterprise) error {
	if err := s.repo.Update(ctx, enterprise); err != nil {
		return fmt.Errorf("failed to update enterprise: %w", err)
	}
	return nil
}

// Delete soft-deletes an enterprise.
func (s *EnterpriseService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete enterprise: %w", err)
	}
	return nil
}
