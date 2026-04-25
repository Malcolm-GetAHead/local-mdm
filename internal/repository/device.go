package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/validation"
)

// DeviceRepository provides data access operations for device management.
// It handles CRUD operations and queries for device resources across all platforms.
type DeviceRepository interface {
	Create(ctx context.Context, device *models.Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error)
	GetBySerial(ctx context.Context, enterpriseID uuid.UUID, serial string) (*models.Device, error)
	GetByPlatformID(ctx context.Context, platform, deviceID string) (*models.Device, error)
	List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error)
	ListFiltered(ctx context.Context, enterpriseID uuid.UUID, platform, status, search string, sortField, sortDir string, limit, offset int) ([]*models.Device, int, error)
	Update(ctx context.Context, device *models.Device) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type deviceRepository struct {
	writer executor
	reader executor
}

// NewDeviceRepository creates a new device repository instance.
// writer is used for Create/Update/Delete, reader for Get/List queries.
// Both must be *sql.DB or an executor interface.
func NewDeviceRepository(writer, reader interface{}) (DeviceRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &deviceRepository{writer: w, reader: r}, nil
}

func (r *deviceRepository) Create(ctx context.Context, device *models.Device) error {
	if err := validation.ValidateJSONB(device.PlatformData, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid platform_data: %w", err)
	}

	query := `
		INSERT INTO devices (id, enterprise_id, platform, device_id, serial_number, name, model, os_version, status, platform_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`
	
	if device.ID == uuid.Nil {
		device.ID = uuid.New()
	}
	
	exec := getExecutor(ctx, r.writer)
	return exec.QueryRowContext(ctx, query,
		device.ID, device.EnterpriseID, device.Platform, device.DeviceID,
		device.SerialNumber, device.Name, device.Model, device.OSVersion,
		device.Status, device.PlatformData,
	).Scan(&device.CreatedAt, &device.UpdatedAt)
}

func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	query := `
		SELECT id, enterprise_id, platform, device_id, serial_number, name, model, os_version,
		       enrollment_date, last_seen, status, platform_data, created_at, updated_at, deleted_at
		FROM devices
		WHERE id = $1 AND deleted_at IS NULL`
	
	device := &models.Device{}
	exec := getReadExecutor(ctx, r.reader)
	err := exec.QueryRowContext(ctx, query, id).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found: %w", apperrors.ErrNotFound)
	}
	return device, err
}

func (r *deviceRepository) GetBySerial(ctx context.Context, enterpriseID uuid.UUID, serial string) (*models.Device, error) {
	query := `
		SELECT id, enterprise_id, platform, device_id, serial_number, name, model, os_version,
		       enrollment_date, last_seen, status, platform_data, created_at, updated_at, deleted_at
		FROM devices
		WHERE enterprise_id = $1 AND serial_number = $2 AND deleted_at IS NULL`
	
	device := &models.Device{}
	exec := getReadExecutor(ctx, r.reader)
	err := exec.QueryRowContext(ctx, query, enterpriseID, serial).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found: %w", apperrors.ErrNotFound)
	}
	return device, err
}

func (r *deviceRepository) GetByPlatformID(ctx context.Context, platform, deviceID string) (*models.Device, error) {
	query := `
		SELECT id, enterprise_id, platform, device_id, serial_number, name, model, os_version,
		       enrollment_date, last_seen, status, platform_data, created_at, updated_at, deleted_at
		FROM devices
		WHERE platform = $1 AND device_id = $2 AND deleted_at IS NULL`

	device := &models.Device{}
	err := getReadExecutor(ctx, r.reader).QueryRowContext(ctx, query, platform, deviceID).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found: %w", apperrors.ErrNotFound)
	}
	return device, err
}

func (r *deviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	// Validate pagination parameters
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	countQuery := `SELECT COUNT(*) FROM devices WHERE enterprise_id = $1 AND deleted_at IS NULL`
	dataQuery := `
		SELECT id, enterprise_id, platform, device_id, serial_number, name, model, os_version,
		       enrollment_date, last_seen, status, platform_data, created_at, updated_at, deleted_at
		FROM devices
		WHERE enterprise_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	scanFn := func(rows *sql.Rows) (*models.Device, error) {
		device := &models.Device{}
		err := rows.Scan(
			&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
			&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
			&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
			&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
		)
		return device, err
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

func (r *deviceRepository) ListFiltered(ctx context.Context, enterpriseID uuid.UUID, platform, status, search string, sortField, sortDir string, limit, offset int) ([]*models.Device, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	where := "WHERE enterprise_id = $1 AND deleted_at IS NULL"
	args := []interface{}{enterpriseID}
	argN := 2

	if platform != "" {
		where += fmt.Sprintf(" AND platform = $%d", argN)
		args = append(args, platform)
		argN++
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, status)
		argN++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (LOWER(name) LIKE $%d OR LOWER(model) LIKE $%d OR LOWER(serial_number) LIKE $%d)", argN, argN, argN)
		args = append(args, "%"+strings.ToLower(search)+"%")
		argN++
	}

	// Validate sort field
	orderCol := "name"
	switch sortField {
	case "name", "platform", "model", "os_version", "status":
		orderCol = sortField
	case "last_seen":
		orderCol = "last_seen"
	}
	order := "ASC"
	if strings.ToLower(sortDir) == "desc" {
		order = "DESC"
	}

	exec := getReadExecutor(ctx, r.reader)
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := exec.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf(`SELECT id, enterprise_id, platform, device_id, serial_number, name, model, os_version,
		enrollment_date, last_seen, status, platform_data, created_at, updated_at, deleted_at
		FROM devices %s ORDER BY %s %s LIMIT $%d OFFSET $%d`, where, orderCol, order, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := exec.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		device := &models.Device{}
		if err := rows.Scan(
			&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
			&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
			&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
			&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
		); err != nil {
			return nil, 0, err
		}
		devices = append(devices, device)
	}
	return devices, total, rows.Err()
}

func (r *deviceRepository) Update(ctx context.Context, device *models.Device) error {
	if err := validation.ValidateJSONB(device.PlatformData, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid platform_data: %w", err)
	}

	query := `
		UPDATE devices
		SET name = $1, model = $2, os_version = $3, last_seen = $4, status = $5, platform_data = $6
		WHERE id = $7 AND deleted_at IS NULL`
	
	exec := getExecutor(ctx, r.writer)
	result, err := exec.ExecContext(ctx, query,
		device.Name, device.Model, device.OSVersion, device.LastSeen,
		device.Status, device.PlatformData, device.ID,
	)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("device not found: %w", apperrors.ErrNotFound)
	}
	
	return nil
}

func (r *deviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE devices SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	exec := getExecutor(ctx, r.writer)
	result, err := exec.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("device not found: %w", apperrors.ErrNotFound)
	}
	
	return nil
}
