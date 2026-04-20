package repository

import (
	"context"
	"database/sql"
	"fmt"

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
	Update(ctx context.Context, device *models.Device) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type deviceRepository struct {
	db executor
}

// NewDeviceRepository creates a new device repository instance.
// The db parameter must be either *sql.DB or an executor interface.
// Returns an error if db is nil or of an unsupported type.
func NewDeviceRepository(db interface{}) (DeviceRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	
	switch v := db.(type) {
	case *sql.DB:
		return &deviceRepository{db: v}, nil
	case executor:
		return &deviceRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
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
	
	exec := getExecutor(ctx, r.db)
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
	exec := getExecutor(ctx, r.db)
	err := exec.QueryRowContext(ctx, query, id).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found")
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
	exec := getExecutor(ctx, r.db)
	err := exec.QueryRowContext(ctx, query, enterpriseID, serial).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found")
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
	err := getExecutor(ctx, r.db).QueryRowContext(ctx, query, platform, deviceID).Scan(
		&device.ID, &device.EnterpriseID, &device.Platform, &device.DeviceID,
		&device.SerialNumber, &device.Name, &device.Model, &device.OSVersion,
		&device.EnrollmentDate, &device.LastSeen, &device.Status, &device.PlatformData,
		&device.CreatedAt, &device.UpdatedAt, &device.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("device not found")
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
		getExecutor(ctx, r.db),
		countQuery,
		[]interface{}{enterpriseID},
		dataQuery,
		[]interface{}{enterpriseID, limit, offset},
		scanFn,
	)
}

func (r *deviceRepository) Update(ctx context.Context, device *models.Device) error {
	if err := validation.ValidateJSONB(device.PlatformData, validation.MaxJSONBDepth); err != nil {
		return fmt.Errorf("invalid platform_data: %w", err)
	}

	query := `
		UPDATE devices
		SET name = $1, model = $2, os_version = $3, last_seen = $4, status = $5, platform_data = $6
		WHERE id = $7 AND deleted_at IS NULL`
	
	exec := getExecutor(ctx, r.db)
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
		return fmt.Errorf("device not found")
	}
	
	return nil
}

func (r *deviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE devices SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	exec := getExecutor(ctx, r.db)
	result, err := exec.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("device not found")
	}
	
	return nil
}
