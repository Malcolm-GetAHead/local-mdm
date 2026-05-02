package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// AuditLogRepository is the interface for audit log data access.
type AuditLogRepository interface {
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.AuditLog, int, error)
	Search(ctx context.Context, enterpriseID uuid.UUID, action, startDate, endDate string, limit, offset int) ([]*models.AuditLog, int, error)
}

// AuditLogService handles audit log retrieval business logic.
type AuditLogService struct {
	repo   AuditLogRepository
	logger *slog.Logger
}

// NewAuditLogService creates a new audit log service.
func NewAuditLogService(repo AuditLogRepository, logger *slog.Logger) *AuditLogService {
	return &AuditLogService{repo: repo, logger: logger}
}

// List returns audit logs for an enterprise.
func (s *AuditLogService) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.AuditLog, int, error) {
	return s.repo.List(ctx, enterpriseID, limit, offset)
}

// Search returns filtered audit logs for an enterprise.
func (s *AuditLogService) Search(ctx context.Context, enterpriseID uuid.UUID, action, startDate, endDate string, limit, offset int) ([]*models.AuditLog, int, error) {
	return s.repo.Search(ctx, enterpriseID, action, startDate, endDate, limit, offset)
}
