package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// AuditLogRepository provides read access to audit log entries.
type AuditLogRepository interface {
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.AuditLog, int, error)
}

type auditLogRepository struct {
	db executor
}

// NewAuditLogRepository creates a new audit log repository instance.
func NewAuditLogRepository(db interface{}) (AuditLogRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	switch v := db.(type) {
	case *sql.DB:
		return &auditLogRepository{db: v}, nil
	case executor:
		return &auditLogRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
}

func (r *auditLogRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.AuditLog, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE enterprise_id = $1`
	dataQuery := `
		SELECT id, enterprise_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE enterprise_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	scanFn := func(rows *sql.Rows) (*models.AuditLog, error) {
		log := &models.AuditLog{}
		var ipAddr sql.NullString
		err := rows.Scan(
			&log.ID, &log.EnterpriseID, &log.UserID, &log.Action, &log.ResourceType,
			&log.ResourceID, &log.Details, &ipAddr, &log.UserAgent, &log.CreatedAt,
		)
		if ipAddr.Valid {
			log.IPAddress = ipAddr.String
		}
		return log, err
	}

	return ExecutePaginatedQuery(
		ctx,
		getExecutor(ctx, r.db),
		countQuery, []interface{}{enterpriseID},
		dataQuery, []interface{}{enterpriseID, limit, offset},
		scanFn,
	)
}
