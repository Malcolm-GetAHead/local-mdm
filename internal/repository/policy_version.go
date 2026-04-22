package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// PolicyVersionRepository provides data access for policy version snapshots.
type PolicyVersionRepository interface {
	Create(ctx context.Context, v *models.PolicyVersion) error
	ListByPolicy(ctx context.Context, policyID uuid.UUID, limit, offset int) ([]*models.PolicyVersion, int, error)
	GetByVersion(ctx context.Context, policyID uuid.UUID, version int) (*models.PolicyVersion, error)
	LatestVersion(ctx context.Context, policyID uuid.UUID) (int, error)
}

type policyVersionRepository struct {
	writer executor
	reader executor
}

// NewPolicyVersionRepository creates a new policy version repository.
// writer is used for Create, reader for Get/List queries.
func NewPolicyVersionRepository(writer, reader interface{}) (PolicyVersionRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &policyVersionRepository{writer: w, reader: r}, nil
}

func (r *policyVersionRepository) Create(ctx context.Context, v *models.PolicyVersion) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}

	configVal, err := v.PolicyConfig.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal policy_config: %w", err)
	}

	_, err = getExecutor(ctx, r.writer).ExecContext(ctx,
		`INSERT INTO policy_versions (id, policy_id, version, policy_config, name, description, platform, policy_type, is_active, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		v.ID, v.PolicyID, v.Version, configVal, v.Name, v.Description, v.Platform, v.PolicyType, v.IsActive, v.CreatedBy,
	)
	return err
}

func (r *policyVersionRepository) ListByPolicy(ctx context.Context, policyID uuid.UUID, limit, offset int) ([]*models.PolicyVersion, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM policy_versions WHERE policy_id = $1`, policyID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, policy_id, version, policy_config, name, description, platform, policy_type, is_active, created_by
		 FROM policy_versions WHERE policy_id = $1 ORDER BY version DESC LIMIT $2 OFFSET $3`,
		policyID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var versions []*models.PolicyVersion
	for rows.Next() {
		v := &models.PolicyVersion{}
		if err := rows.Scan(&v.ID, &v.PolicyID, &v.Version, &v.PolicyConfig, &v.Name, &v.Description, &v.Platform, &v.PolicyType, &v.IsActive, &v.CreatedBy); err != nil {
			return nil, 0, err
		}
		versions = append(versions, v)
	}
	return versions, total, rows.Err()
}

func (r *policyVersionRepository) GetByVersion(ctx context.Context, policyID uuid.UUID, version int) (*models.PolicyVersion, error) {
	v := &models.PolicyVersion{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, policy_id, version, policy_config, name, description, platform, policy_type, is_active, created_by
		 FROM policy_versions WHERE policy_id = $1 AND version = $2`,
		policyID, version,
	).Scan(&v.ID, &v.PolicyID, &v.Version, &v.PolicyConfig, &v.Name, &v.Description, &v.Platform, &v.PolicyType, &v.IsActive, &v.CreatedBy)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("policy version not found: %w", apperrors.ErrNotFound)
	}
	return v, err
}

func (r *policyVersionRepository) LatestVersion(ctx context.Context, policyID uuid.UUID) (int, error) {
	var version int
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM policy_versions WHERE policy_id = $1`, policyID,
	).Scan(&version)
	return version, err
}
