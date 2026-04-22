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
	Search(ctx context.Context, enterpriseID uuid.UUID, action, startDate, endDate string, limit, offset int) ([]*models.AuditLog, int, error)
}

type auditLogRepository struct {
	writer executor
	reader executor
}

// NewAuditLogRepository creates a new audit log repository instance.
// writer is used for Create operations, reader for List queries.
func NewAuditLogRepository(writer, reader interface{}) (AuditLogRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &auditLogRepository{writer: w, reader: r}, nil
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
		getReadExecutor(ctx, r.reader),
		countQuery, []interface{}{enterpriseID},
		dataQuery, []interface{}{enterpriseID, limit, offset},
		scanFn,
	)
}

func (r *auditLogRepository) Search(ctx context.Context, enterpriseID uuid.UUID, action, startDate, endDate string, limit, offset int) ([]*models.AuditLog, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	where := "WHERE enterprise_id = $1"
	args := []interface{}{enterpriseID}
	argN := 2

	if action != "" {
		where += fmt.Sprintf(" AND action LIKE $%d", argN)
		args = append(args, "%"+action+"%")
		argN++
	}
	if startDate != "" {
		where += fmt.Sprintf(" AND created_at >= $%d", argN)
		args = append(args, startDate)
		argN++
	}
	if endDate != "" {
		where += fmt.Sprintf(" AND created_at <= $%d", argN)
		args = append(args, endDate)
		argN++
	}

	exec := getReadExecutor(ctx, r.reader)
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := exec.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`SELECT id, enterprise_id, user_id, action, resource_type, resource_id,
		details, ip_address, user_agent, created_at FROM audit_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var ipAddr sql.NullString
		if err := rows.Scan(&log.ID, &log.EnterpriseID, &log.UserID, &log.Action, &log.ResourceType,
			&log.ResourceID, &log.Details, &ipAddr, &log.UserAgent, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		if ipAddr.Valid {
			log.IPAddress = ipAddr.String
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}
