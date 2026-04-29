package certs

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CertificateService provides certificate management operations for device enrollment.
// It handles certificate issuance, storage, retrieval, and expiration tracking.
type CertificateService struct {
	ca    *CAManager
	store CertStore
}

// NewCertificateService creates a new certificate service instance.
// The service handles certificate issuance, storage, and retrieval for device enrollment.
func NewCertificateService(ca *CAManager, store CertStore) *CertificateService {
	return &CertificateService{
		ca:    ca,
		store: store,
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
		// Windows CSRs may contain non-PrintableString characters that Go's
		// strict ASN.1 parser rejects. Fall back to signing with a generic subject.
		cert, signErr := s.ca.SignRawCSR(block.Bytes, validity)
		if signErr != nil {
			return nil, fmt.Errorf("failed to parse CSR: %w (fallback also failed: %v)", err, signErr)
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		if storeErr := s.store.StoreCertificate(ctx, uuid.New(), deviceID, "device",
			cert.Subject.CommonName, cert.SerialNumber.String(), string(certPEM),
			cert.NotBefore, cert.NotAfter); storeErr != nil {
			return nil, fmt.Errorf("failed to store certificate: %w", storeErr)
		}
		return certPEM, nil
	}

	// Sign certificate
	cert, err := s.ca.SignCSR(csr, validity)
	if err != nil {
		return nil, fmt.Errorf("failed to sign CSR: %w", err)
	}

	// Store in database
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	if err := s.store.StoreCertificate(ctx,
		uuid.New(),
		deviceID,
		"device",
		cert.Subject.CommonName,
		cert.SerialNumber.String(),
		string(certPEM),
		cert.NotBefore,
		cert.NotAfter,
	); err != nil {
		return nil, fmt.Errorf("failed to store certificate: %w", err)
	}

	return certPEM, nil
}

func (s *CertificateService) RevokeCertificate(ctx context.Context, serialNumber string) error {
	return s.store.RevokeCertificate(ctx, serialNumber)
}

func (s *CertificateService) GetCertificateBySerial(ctx context.Context, serialNumber string) (*models.Certificate, error) {
	return s.store.GetCertificateBySerial(ctx, serialNumber)
}
