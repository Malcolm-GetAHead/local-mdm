package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/validation"
)

// PolicyRepository provides data access operations for policy management.
// It handles policy CRUD operations and device-policy assignments.
type PolicyRepository interface {
	Create(ctx context.Context, policy *models.Policy) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Policy, int, error)
	Update(ctx context.Context, policy *models.Policy) error
	Delete(ctx context.Context, id uuid.UUID) error
	AssignToDevice(ctx context.Context, deviceID, policyID uuid.UUID) error
	UnassignFromDevice(ctx context.Context, deviceID, policyID uuid.UUID) error
}

type policyRepository struct {
	writer executor
	reader executor
}

// NewPolicyRepository creates a new policy repository instance.
// writer is used for Create/Update/Delete, reader for Get/List queries.
// Both must be *sql.DB or an executor interface.
func NewPolicyRepository(writer, reader interface{}) (PolicyRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &policyRepository{writer: w, reader: r}, nil
}

func (r *policyRepository) Create(ctx context.Context, policy *models.Policy) error {
	if err := validation.ValidateJSONB(policy.PolicyConfig, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid policy_config: %w", err)
	}

	query := `
		INSERT INTO policies (id, enterprise_id, name, description, platform, policy_type, policy_config, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`
	
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	
	return getExecutor(ctx, r.writer).QueryRowContext(ctx, query,
		policy.ID, policy.EnterpriseID, policy.Name, policy.Description,
		policy.Platform, policy.PolicyType, policy.PolicyConfig, policy.IsActive,
	).Scan(&policy.CreatedAt, &policy.UpdatedAt)
}

func (r *policyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error) {
	query := `
		SELECT id, enterprise_id, name, description, platform, policy_type, policy_config, is_active,
		       created_at, updated_at, deleted_at
		FROM policies
		WHERE id = $1 AND deleted_at IS NULL`
	
	policy := &models.Policy{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx, query, id).Scan(
		&policy.ID, &policy.EnterpriseID, &policy.Name, &policy.Description,
		&policy.Platform, &policy.PolicyType, &policy.PolicyConfig, &policy.IsActive,
		&policy.CreatedAt, &policy.UpdatedAt, &policy.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("policy not found")
	}
	return policy, err
}

func (r *policyRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Policy, int, error) {
	// Validate pagination parameters
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	countQuery := `SELECT COUNT(*) FROM policies WHERE enterprise_id = $1 AND deleted_at IS NULL`
	dataQuery := `
		SELECT id, enterprise_id, name, description, platform, policy_type, policy_config, is_active,
		       created_at, updated_at, deleted_at
		FROM policies
		WHERE enterprise_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	scanFn := func(rows *sql.Rows) (*models.Policy, error) {
		policy := &models.Policy{}
		err := rows.Scan(
			&policy.ID, &policy.EnterpriseID, &policy.Name, &policy.Description,
			&policy.Platform, &policy.PolicyType, &policy.PolicyConfig, &policy.IsActive,
			&policy.CreatedAt, &policy.UpdatedAt, &policy.DeletedAt,
		)
		return policy, err
	}

	return ExecutePaginatedQuery(
		ctx,
		getReadExecutor(ctx, r.reader),
		countQuery,
		[]interface{}{enterpriseID},
		dataQuery,
		[]interface{}{enterpriseID, limit, offset},
		scanFn,
	)
}

func (r *policyRepository) Update(ctx context.Context, policy *models.Policy) error {
	if err := validation.ValidateJSONB(policy.PolicyConfig, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid policy_config: %w", err)
	}

	query := `
		UPDATE policies
		SET name = $1, description = $2, policy_config = $3, is_active = $4
		WHERE id = $5 AND deleted_at IS NULL`
	
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx, query,
		policy.Name, policy.Description, policy.PolicyConfig, policy.IsActive, policy.ID,
	)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("policy not found")
	}
	
	return nil
}

func (r *policyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE policies SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("policy not found")
	}
	
	return nil
}

func (r *policyRepository) AssignToDevice(ctx context.Context, deviceID, policyID uuid.UUID) error {
	query := `
		INSERT INTO device_policies (id, device_id, policy_id, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (device_id, policy_id) DO NOTHING`
	
	_, err := getExecutor(ctx, r.writer).ExecContext(ctx, query, uuid.New(), deviceID, policyID)
	return err
}

func (r *policyRepository) UnassignFromDevice(ctx context.Context, deviceID, policyID uuid.UUID) error {
	query := `DELETE FROM device_policies WHERE device_id = $1 AND policy_id = $2`
	_, err := getExecutor(ctx, r.writer).ExecContext(ctx, query, deviceID, policyID)
	return err
}
