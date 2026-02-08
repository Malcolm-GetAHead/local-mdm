package certs

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertificateService provides certificate management operations for device enrollment.
// It handles certificate issuance, storage, retrieval, and expiration tracking.
type CertificateService struct {
	ca *CAManager
	db *sql.DB
}

// NewCertificateService creates a new certificate service instance.
// The service handles certificate issuance, storage, and retrieval for device enrollment.
func NewCertificateService(ca *CAManager, db *sql.DB) *CertificateService {
	return &CertificateService{
		ca: ca,
		db: db,
	}
}

func (s *CertificateService) GetCACertificate() (*x509.Certificate, error) {
	return s.ca.GetCACertificate(), nil
}

func (s *CertificateService) GetCACertificatePEM() ([]byte, error) {
	return s.ca.GetCACertificatePEM()
}

func (s *CertificateService) SignDeviceCSR(ctx context.Context, deviceID uuid.UUID, csrPEM []byte, validity time.Duration) ([]byte, error) {
	// Parse CSR
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CSR PEM")
	}
	
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR: %w", err)
	}
	
	// Sign certificate
	cert, err := s.ca.SignCSR(csr, validity)
	if err != nil {
		return nil, fmt.Errorf("failed to sign CSR: %w", err)
	}
	
	// Store in database
	if err := s.storeCertificate(ctx, deviceID, cert); err != nil {
		return nil, fmt.Errorf("failed to store certificate: %w", err)
	}
	
	// Return PEM
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}), nil
}

func (s *CertificateService) storeCertificate(ctx context.Context, deviceID uuid.UUID, cert *x509.Certificate) error {
	query := `
		INSERT INTO certificates (id, device_id, cert_type, subject, serial_number, cert_data, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
	
	_, err := s.db.ExecContext(ctx, query,
		uuid.New(),
		deviceID,
		"device",
		cert.Subject.CommonName,
		cert.SerialNumber.String(),
		string(certPEM),
		cert.NotBefore,
		cert.NotAfter,
	)
	
	return err
}

func (s *CertificateService) RevokeCertificate(ctx context.Context, serialNumber string) error {
	query := `UPDATE certificates SET revoked_at = NOW() WHERE serial_number = $1 AND revoked_at IS NULL`
	result, err := s.db.ExecContext(ctx, query, serialNumber)
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

func (s *CertificateService) GetCertificateBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	query := `
		SELECT id, device_id, cert_type, subject, serial_number, cert_data, issued_at, expires_at, revoked_at, created_at, updated_at
		FROM certificates
		WHERE serial_number = $1`
	
	cert := &models.Certificate{}
	var deviceID sql.NullString
	
	err := s.db.QueryRowContext(ctx, query, serialNumber).Scan(
		&cert.ID, &deviceID, &cert.CertType, &cert.Subject, &cert.SerialNumber,
		&cert.CertData, &cert.IssuedAt, &cert.ExpiresAt, &cert.RevokedAt,
		&cert.CreatedAt, &cert.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("certificate not found")
	}
	
	if deviceID.Valid {
		id, _ := uuid.Parse(deviceID.String)
		cert.DeviceID = &id
	}
	
	return cert, err
}
