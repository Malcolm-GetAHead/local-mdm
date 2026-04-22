package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/apperrors"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertificateRepository provides data access operations for certificate management.
type CertificateRepository interface {
	GetBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error)
	List(ctx context.Context, deviceID *uuid.UUID, limit, offset int) ([]*models.Certificate, int, error)
}

type certificateRepository struct {
	writer executor
	reader executor
}

// NewCertificateRepository creates a new certificate repository instance.
// writer is used for Create/Revoke, reader for Get/List queries.
func NewCertificateRepository(writer, reader interface{}) (CertificateRepository, error) {
	w, err := resolveExecutor(writer, "writer")
	if err != nil {
		return nil, err
	}
	r, err := resolveExecutor(reader, "reader")
	if err != nil {
		return nil, err
	}
	return &certificateRepository{writer: w, reader: r}, nil
}

func (r *certificateRepository) GetBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	query := `
		SELECT id, device_id, cert_type, subject, serial_number, cert_data,
		       issued_at, expires_at, revoked_at, created_at, updated_at
		FROM certificates
		WHERE serial_number = $1`

	cert := &models.Certificate{}
	exec := getReadExecutor(ctx, r.reader)
	err := exec.QueryRowContext(ctx, query, serialNumber).Scan(
		&cert.ID, &cert.DeviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
		&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
		&cert.CreatedAt, &cert.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("certificate not found: %w", apperrors.ErrNotFound)
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
		getReadExecutor(ctx, r.reader),
		countQuery, countArgs,
		dataQuery, dataArgs,
		scanFn,
	)
}
