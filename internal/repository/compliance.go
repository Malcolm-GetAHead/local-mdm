package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// ComplianceRepository provides data access for compliance results.
type ComplianceRepository interface {
	Upsert(ctx context.Context, result *models.ComplianceResult) error
	GetByDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error)
	GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error)
}

type complianceRepository struct {
	db executor
}

func NewComplianceRepository(db interface{}) (ComplianceRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	switch v := db.(type) {
	case *sql.DB:
		return &complianceRepository{db: v}, nil
	case executor:
		return &complianceRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
}

func (r *complianceRepository) Upsert(ctx context.Context, result *models.ComplianceResult) error {
	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}

	detailsVal, err := result.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	return getExecutor(ctx, r.db).QueryRowContext(ctx,
		`INSERT INTO compliance_results (id, device_id, policy_id, status, details, evaluated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (device_id, policy_id) DO UPDATE SET status = $4, details = $5, evaluated_at = NOW()
		 RETURNING evaluated_at`,
		result.ID, result.DeviceID, result.PolicyID, result.Status, detailsVal,
	).Scan(&result.EvaluatedAt)
}

func (r *complianceRepository) GetByDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.ComplianceResult, error) {
	rows, err := getExecutor(ctx, r.db).QueryContext(ctx,
		`SELECT id, device_id, policy_id, status, details, evaluated_at
		 FROM compliance_results WHERE device_id = $1 ORDER BY evaluated_at DESC`, deviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.ComplianceResult
	for rows.Next() {
		cr := &models.ComplianceResult{}
		if err := rows.Scan(&cr.ID, &cr.DeviceID, &cr.PolicyID, &cr.Status, &cr.Details, &cr.EvaluatedAt); err != nil {
			return nil, err
		}
		results = append(results, cr)
	}
	return results, rows.Err()
}

func (r *complianceRepository) GetSummary(ctx context.Context, enterpriseID uuid.UUID) (*models.ComplianceSummary, error) {
	summary := &models.ComplianceSummary{}
	rows, err := getExecutor(ctx, r.db).QueryContext(ctx,
		`SELECT cr.status, COUNT(*)
		 FROM compliance_results cr
		 JOIN devices d ON cr.device_id = d.id
		 WHERE d.enterprise_id = $1 AND d.deleted_at IS NULL
		 GROUP BY cr.status`, enterpriseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case models.ComplianceStatusCompliant:
			summary.Compliant = count
		case models.ComplianceStatusNonCompliant:
			summary.NonCompliant = count
		case models.ComplianceStatusUnknown:
			summary.Unknown = count
		case models.ComplianceStatusError:
			summary.Error = count
		}
		summary.Total += count
	}
	return summary, rows.Err()
}
