package certs_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
)

func setupTestCerts(t *testing.T) (*certs.CAManager, *certs.CertificateService, *db.DB) {
	// Create temp directory for CA files
	tmpDir := t.TempDir()
	certPath := tmpDir + "/ca.crt"
	keyPath := tmpDir + "/ca.key"
	
	// Generate CA explicitly
	ca, err := certs.GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}
	
	database := testutil.ConnectDB(t)
	
	// Create certificate service
	service := certs.NewCertificateService(ca, database.Writer)
	
	return ca, service, database
}

func createTestDevice(t *testing.T, database *db.DB) uuid.UUID {
	// Create enterprise first
	enterpriseID := uuid.New()
	_, err := database.Writer.ExecContext(context.Background(),
		`INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)`,
		enterpriseID, "Test Enterprise", "test-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("Failed to create enterprise: %v", err)
	}
	
	// Create device
	deviceID := uuid.New()
	_, err = database.Writer.ExecContext(context.Background(),
		`INSERT INTO devices (id, enterprise_id, platform, device_id, serial_number, name, status) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		deviceID, enterpriseID, "macos", "test-"+uuid.New().String(), "SN"+uuid.New().String()[:8], "Test Device", "enrolled")
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	
	return deviceID
}

func TestCAGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := tmpDir + "/ca.crt"
	keyPath := tmpDir + "/ca.key"
	
	// Generate CA explicitly
	ca, err := certs.GenerateCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}
	
	// Verify CA certificate
	cert := ca.GetCACertificate()
	if cert == nil {
		t.Fatal("CA certificate is nil")
	}
	
	if !cert.IsCA {
		t.Error("Certificate should be a CA")
	}
	
	if cert.Subject.CommonName != "Local MDM Root CA" {
		t.Errorf("Expected CN 'Local MDM Root CA', got '%s'", cert.Subject.CommonName)
	}
	
	// Verify files were created
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("CA certificate file was not created")
	}
	
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("CA key file was not created")
	}
	
	// Test loading existing CA via NewCAManager
	ca2, err := certs.NewCAManager(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to load existing CA: %v", err)
	}
	
	cert2 := ca2.GetCACertificate()
	if cert.SerialNumber.Cmp(cert2.SerialNumber) != 0 {
		t.Error("Loaded CA should have same serial number")
	}

	// Test NewCAManager fails on missing files
	_, err = certs.NewCAManager(tmpDir+"/nonexistent.crt", tmpDir+"/nonexistent.key")
	if err == nil {
		t.Error("NewCAManager should fail on missing files")
	}

	// Test GenerateCA refuses to overwrite
	_, err = certs.GenerateCA(certPath, keyPath)
	if err == nil {
		t.Error("GenerateCA should refuse to overwrite existing files")
	}
}

func TestCSRSigning(t *testing.T) {
	ca, service, database := setupTestCerts(t)
	
	// Create test device
	deviceID := createTestDevice(t, database)
	
	// Generate a CSR
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: "Test Device",
		},
	}
	
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatalf("Failed to create CSR: %v", err)
	}
	
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})
	
	// Sign CSR
	certPEM, err := service.SignDeviceCSR(context.Background(), deviceID, csrPEM, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to sign CSR: %v", err)
	}
	
	// Parse signed certificate
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode certificate PEM")
	}
	
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	
	// Verify certificate
	if cert.Subject.CommonName != "Test Device" {
		t.Errorf("Expected CN 'Test Device', got '%s'", cert.Subject.CommonName)
	}
	
	// Verify certificate is signed by CA
	caCert := ca.GetCACertificate()
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Certificate signature verification failed: %v", err)
	}
	
	// Verify certificate was stored in database
	storedCert, err := service.GetCertificateBySerial(context.Background(), cert.SerialNumber.String())
	if err != nil {
		t.Fatalf("Failed to retrieve certificate from database: %v", err)
	}
	
	if storedCert.Subject != "Test Device" {
		t.Error("Stored certificate subject doesn't match")
	}
}

func TestCertificateRevocation(t *testing.T) {
	_, service, database := setupTestCerts(t)
	
	// Create test device
	deviceID := createTestDevice(t, database)
	
	// Generate and sign a certificate
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "Revoke Test"},
	}
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, template, key)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	
	certPEM, err := service.SignDeviceCSR(context.Background(), deviceID, csrPEM, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to sign CSR: %v", err)
	}
	
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)
	serialNumber := cert.SerialNumber.String()
	
	// Revoke certificate
	err = service.RevokeCertificate(context.Background(), serialNumber)
	if err != nil {
		t.Fatalf("Failed to revoke certificate: %v", err)
	}
	
	// Verify revocation
	storedCert, err := service.GetCertificateBySerial(context.Background(), serialNumber)
	if err != nil {
		t.Fatalf("Failed to retrieve certificate: %v", err)
	}
	
	if storedCert.RevokedAt == nil {
		t.Error("Certificate should be marked as revoked")
	}
	
	// Try to revoke again (should fail)
	err = service.RevokeCertificate(context.Background(), serialNumber)
	if err == nil {
		t.Error("Should not be able to revoke already revoked certificate")
	}
}

func TestGetCACertificatePEM(t *testing.T) {
	_, service, _ := setupTestCerts(t)
	
	certPEM, err := service.GetCACertificatePEM()
	if err != nil {
		t.Fatalf("Failed to get CA certificate PEM: %v", err)
	}
	
	// Parse PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode CA certificate PEM")
	}
	
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse CA certificate: %v", err)
	}
	
	if !cert.IsCA {
		t.Error("Certificate should be a CA")
	}
}

func TestNewCAManagerFromPEM(t *testing.T) {
	// Generate a CA via explicit generation first, then load from PEM
	dir := t.TempDir()
	fileCA, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	certPEM, err := os.ReadFile(dir + "/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := os.ReadFile(dir + "/ca.key")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid PEM", func(t *testing.T) {
		pemCA, err := certs.NewCAManagerFromPEM(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("NewCAManagerFromPEM failed: %v", err)
		}
		if pemCA.GetCACertificate().Subject.CommonName != fileCA.GetCACertificate().Subject.CommonName {
			t.Errorf("CN mismatch: got %s, want %s", pemCA.GetCACertificate().Subject.CommonName, fileCA.GetCACertificate().Subject.CommonName)
		}
		if pemCA.GetCAPrivateKey() == nil {
			t.Error("private key is nil")
		}
	})

	t.Run("invalid cert PEM", func(t *testing.T) {
		_, err := certs.NewCAManagerFromPEM([]byte("not a cert"), keyPEM)
		if err == nil {
			t.Error("expected error for invalid cert PEM")
		}
	})

	t.Run("invalid key PEM", func(t *testing.T) {
		_, err := certs.NewCAManagerFromPEM(certPEM, []byte("not a key"))
		if err == nil {
			t.Error("expected error for invalid key PEM")
		}
	})
}
