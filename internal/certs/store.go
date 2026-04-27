package certs

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertStore abstracts certificate persistence for the CertificateService.
type CertStore interface {
	StoreCertificate(ctx context.Context, id, deviceID uuid.UUID, certType, subject, serialNumber, certData string, issuedAt, expiresAt time.Time) error
	RevokeCertificate(ctx context.Context, serialNumber string) error
	GetCertificateBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error)
}

// SQLCertStore implements CertStore using a SQL database.
type SQLCertStore struct {
	db *sql.DB
}

// NewSQLCertStore creates a new SQL-backed certificate store.
func NewSQLCertStore(db *sql.DB) *SQLCertStore {
	return &SQLCertStore{db: db}
}

func (s *SQLCertStore) StoreCertificate(ctx context.Context, id, deviceID uuid.UUID, certType, subject, serialNumber, certData string, issuedAt, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO certificates (id, device_id, cert_type, subject, serial_number, cert_data, issued_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, deviceID, certType, subject, serialNumber, certData, issuedAt, expiresAt)
	return err
}

func (s *SQLCertStore) RevokeCertificate(ctx context.Context, serialNumber string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE certificates SET revoked_at = NOW() WHERE serial_number = $1 AND revoked_at IS NULL`, serialNumber)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("certificate not found or already revoked")
	}
	return nil
}

func (s *SQLCertStore) GetCertificateBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	cert := &models.Certificate{}
	var deviceID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, device_id, cert_type, subject, serial_number, cert_data, issued_at, expires_at, revoked_at, created_at, updated_at
		 FROM certificates WHERE serial_number = $1`, serialNumber,
	).Scan(&cert.ID, &deviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
		&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
		&cert.CreatedAt, &cert.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("certificate not found")
	}
	if err != nil {
		return nil, err
	}
	if deviceID.Valid {
		id, _ := uuid.Parse(deviceID.String)
		cert.DeviceID = &id
	}
	return cert, nil
}
