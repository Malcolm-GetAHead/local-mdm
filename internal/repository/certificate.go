package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertificateRepository provides data access operations for certificate management.
type CertificateRepository interface {
	GetBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error)
	List(ctx context.Context, deviceID *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error)
}

type certificateRepository struct {
	db executor
}

// NewCertificateRepository creates a new certificate repository instance.
func NewCertificateRepository(db interface{}) (CertificateRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	switch v := db.(type) {
	case *sql.DB:
		return &certificateRepository{db: v}, nil
	case executor:
		return &certificateRepository{db: v}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %T", db)
	}
}

func (r *certificateRepository) GetBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	query := `
		SELECT id, device_id, cert_type, subject, serial_number, cert_data,
		       issued_at, expires_at, revoked_at, created_at, updated_at
		FROM certificates
		WHERE serial_number = $1`

	cert := &models.Certificate{}
	exec := getExecutor(ctx, r.db)
	err := exec.QueryRowContext(ctx, query, serialNumber).Scan(
		&cert.ID, &cert.DeviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
		&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
		&cert.CreatedAt, &cert.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("certificate not found")
	}
	return cert, err
}

func (r *certificateRepository) List(ctx context.Context, deviceID *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error) {
	limit, offset, err := ValidatePagination(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pagination: %w", err)
	}

	var countQuery, dataQuery string
	var countArgs, dataArgs []interface{}

	if deviceID != nil {
		countQuery = `SELECT COUNT(*) FROM certificates WHERE device_id = $1`
		countArgs = []interface{}{*deviceID}
		dataQuery = `
			SELECT id, device_id, cert_type, subject, serial_number, cert_data,
			       issued_at, expires_at, revoked_at, created_at, updated_at
			FROM certificates
			WHERE device_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		dataArgs = []interface{}{*deviceID, limit, offset}
	} else {
		countQuery = `SELECT COUNT(*) FROM certificates`
		countArgs = nil
		dataQuery = `
			SELECT id, device_id, cert_type, subject, serial_number, cert_data,
			       issued_at, expires_at, revoked_at, created_at, updated_at
			FROM certificates
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`
		dataArgs = []interface{}{limit, offset}
	}

	scanFn := func(rows *sql.Rows) (*models.Certificate, error) {
		cert := &models.Certificate{}
		err := rows.Scan(
			&cert.ID, &cert.DeviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
			&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
			&cert.CreatedAt, &cert.UpdatedAt,
		)
		return cert, err
	}

	return ExecutePaginatedQuery(
		ctx,
		getExecutor(ctx, r.db),
		countQuery, countArgs,
		dataQuery, dataArgs,
		scanFn,
	)
}
