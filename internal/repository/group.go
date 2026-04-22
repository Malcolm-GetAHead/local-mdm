package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// GroupRepository provides data access for device groups and memberships.
type GroupRepository interface {
	Create(ctx context.Context, group *models.DeviceGroup) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceGroup, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.DeviceGroup, int, error)
	Update(ctx context.Context, group *models.DeviceGroup) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, groupID, deviceID uuid.UUID) error
	RemoveMember(ctx context.Context, groupID, deviceID uuid.UUID) error
	ListMembers(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*models.Device, int, error)
	ListGroupsForDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceGroup, error)
}

// PolicyAssignmentRepository provides data access for policy assignments.
type PolicyAssignmentRepository interface {
	Create(ctx context.Context, a *models.PolicyAssignment) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]*models.PolicyAssignment, error)
	ListByPolicy(ctx context.Context, policyID uuid.UUID) ([]*models.PolicyAssignment, error)
	GetEffectivePolicies(ctx context.Context, deviceID uuid.UUID, groupIDs []uuid.UUID, enterpriseID uuid.UUID) ([]*models.PolicyAssignment, error)
}

type groupRepository struct {
	writer executor
	reader executor
}

func NewGroupRepository(writer, reader interface{}) (GroupRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &groupRepository{writer: w, reader: r}, nil
}

func (r *groupRepository) Create(ctx context.Context, group *models.DeviceGroup) error {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	return getExecutor(ctx, r.writer).QueryRowContext(ctx,
		`INSERT INTO device_groups (id, enterprise_id, name, description) VALUES ($1, $2, $3, $4) RETURNING created_at, updated_at`,
		group.ID, group.EnterpriseID, group.Name, group.Description,
	).Scan(&group.CreatedAt, &group.UpdatedAt)
}

func (r *groupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceGroup, error) {
	g := &models.DeviceGroup{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT id, enterprise_id, name, description, created_at, updated_at FROM device_groups WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&g.ID, &g.EnterpriseID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found: %w", apperrors.ErrNotFound)
	}
	return g, err
}

func (r *groupRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.DeviceGroup, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM device_groups WHERE enterprise_id = $1 AND deleted_at IS NULL`, enterpriseID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, enterprise_id, name, description, created_at, updated_at FROM device_groups
		 WHERE enterprise_id = $1 AND deleted_at IS NULL ORDER BY name LIMIT $2 OFFSET $3`,
		enterpriseID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var groups []*models.DeviceGroup
	for rows.Next() {
		g := &models.DeviceGroup{}
		if err := rows.Scan(&g.ID, &g.EnterpriseID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, err
		}
		groups = append(groups, g)
	}
	return groups, total, rows.Err()
}

func (r *groupRepository) Update(ctx context.Context, group *models.DeviceGroup) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE device_groups SET name = $1, description = $2 WHERE id = $3 AND deleted_at IS NULL`,
		group.Name, group.Description, group.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *groupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`UPDATE device_groups SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("group not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *groupRepository) AddMember(ctx context.Context, groupID, deviceID uuid.UUID) error {
	_, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`INSERT INTO group_memberships (group_id, device_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		groupID, deviceID,
	)
	return err
}

func (r *groupRepository) RemoveMember(ctx context.Context, groupID, deviceID uuid.UUID) error {
	_, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`DELETE FROM group_memberships WHERE group_id = $1 AND device_id = $2`, groupID, deviceID,
	)
	return err
}

func (r *groupRepository) ListMembers(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = getReadExecutor(ctx, r.reader).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_memberships WHERE group_id = $1`, groupID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT d.id, d.enterprise_id, d.platform, d.device_id, d.serial_number, d.name, d.model,
		        d.os_version, d.enrollment_date, d.last_seen, d.status, d.platform_data, d.created_at, d.updated_at, d.deleted_at
		 FROM devices d JOIN group_memberships gm ON d.id = gm.device_id
		 WHERE gm.group_id = $1 AND d.deleted_at IS NULL ORDER BY d.name LIMIT $2 OFFSET $3`,
		groupID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		d := &models.Device{}
		if err := rows.Scan(&d.ID, &d.EnterpriseID, &d.Platform, &d.DeviceID, &d.SerialNumber,
			&d.Name, &d.Model, &d.OSVersion, &d.EnrollmentDate, &d.LastSeen, &d.Status,
			&d.PlatformData, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt); err != nil {
			return nil, 0, err
		}
		devices = append(devices, d)
	}
	return devices, total, rows.Err()
}

func (r *groupRepository) ListGroupsForDevice(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceGroup, error) {
	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT g.id, g.enterprise_id, g.name, g.description, g.created_at, g.updated_at
		 FROM device_groups g JOIN group_memberships gm ON g.id = gm.group_id
		 WHERE gm.device_id = $1 AND g.deleted_at IS NULL`, deviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*models.DeviceGroup
	for rows.Next() {
		g := &models.DeviceGroup{}
		if err := rows.Scan(&g.ID, &g.EnterpriseID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// --- Policy Assignment Repository ---

type policyAssignmentRepository struct {
	writer executor
	reader executor
}

func NewPolicyAssignmentRepository(writer, reader interface{}) (PolicyAssignmentRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &policyAssignmentRepository{writer: w, reader: r}, nil
}

func (r *policyAssignmentRepository) Create(ctx context.Context, a *models.PolicyAssignment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return getExecutor(ctx, r.writer).QueryRowContext(ctx,
		`INSERT INTO policy_assignments (id, policy_id, target_type, target_id, priority)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (policy_id, target_type, target_id) DO NOTHING RETURNING created_at`,
		a.ID, a.PolicyID, a.TargetType, a.TargetID, a.Priority,
	).Scan(&a.CreatedAt)
}

func (r *policyAssignmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := getExecutor(ctx, r.writer).ExecContext(ctx,
		`DELETE FROM policy_assignments WHERE id = $1`, id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("assignment not found: %w", apperrors.ErrNotFound)
	}
	return nil
}

func (r *policyAssignmentRepository) ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]*models.PolicyAssignment, error) {
	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, policy_id, target_type, target_id, priority, created_at
		 FROM policy_assignments WHERE target_type = $1 AND target_id = $2 ORDER BY priority, created_at`,
		targetType, targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssignments(rows)
}

func (r *policyAssignmentRepository) ListByPolicy(ctx context.Context, policyID uuid.UUID) ([]*models.PolicyAssignment, error) {
	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx,
		`SELECT id, policy_id, target_type, target_id, priority, created_at
		 FROM policy_assignments WHERE policy_id = $1 ORDER BY priority, created_at`, policyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssignments(rows)
}

func (r *policyAssignmentRepository) GetEffectivePolicies(ctx context.Context, deviceID uuid.UUID, groupIDs []uuid.UUID, enterpriseID uuid.UUID) ([]*models.PolicyAssignment, error) {
	// Build target list: device itself + all its groups + enterprise
	query := `SELECT DISTINCT ON (policy_id) id, policy_id, target_type, target_id, priority, created_at
		FROM policy_assignments
		WHERE (target_type = 'device' AND target_id = $1)
		   OR (target_type = 'enterprise' AND target_id = $2)`

	args := []interface{}{deviceID, enterpriseID}

	if len(groupIDs) > 0 {
		// Add group targets
		query += ` OR (target_type = 'group' AND target_id = ANY($3))`
		args = append(args, groupIDs)
	}

	query += ` ORDER BY policy_id, priority, created_at`

	rows, err := getReadExecutor(ctx, r.reader).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssignments(rows)
}

func scanAssignments(rows *sql.Rows) ([]*models.PolicyAssignment, error) {
	var assignments []*models.PolicyAssignment
	for rows.Next() {
		a := &models.PolicyAssignment{}
		if err := rows.Scan(&a.ID, &a.PolicyID, &a.TargetType, &a.TargetID, &a.Priority, &a.CreatedAt); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}
