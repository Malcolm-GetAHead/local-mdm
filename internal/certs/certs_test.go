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
	service := certs.NewCertificateService(ca, certs.NewSQLCertStore(database.Writer))
	
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

	// Verify CRL was created alongside CA
	crlPath := tmpDir + "/ca.crl"
	if _, err := os.Stat(crlPath); os.IsNotExist(err) {
		t.Error("CRL file was not created alongside CA")
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

func TestSignRawCSR_FallbackSubject(t *testing.T) {
	// SignRawCSR is the fallback path for Windows CSRs that contain
	// non-PrintableString characters Go's x509.ParseCertificateRequest rejects.
	// It should produce a valid cert with CN=MDMDeviceCert.
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	// Create a normal CSR (SignRawCSR works on any DER CSR)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "Windows Device with Ünïcödé"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatalf("failed to create CSR: %v", err)
	}

	cert, err := ca.SignRawCSR(csrDER, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("SignRawCSR failed: %v", err)
	}

	// Fallback uses generic subject
	if cert.Subject.CommonName != "MDMDeviceCert" {
		t.Errorf("expected CN 'MDMDeviceCert', got '%s'", cert.Subject.CommonName)
	}

	// Cert should be signed by our CA
	if err := cert.CheckSignatureFrom(ca.GetCACertificate()); err != nil {
		t.Errorf("certificate signature verification failed: %v", err)
	}

	// Cert should have client auth EKU
	found := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			found = true
			break
		}
	}
	if !found {
		t.Error("certificate missing ClientAuth extended key usage")
	}
}

func TestCertTemplate(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	tmpl := ca.CertTemplate(24 * time.Hour)

	if tmpl.SerialNumber == nil {
		t.Fatal("serial number is nil")
	}
	if tmpl.IsCA {
		t.Error("template should not be CA")
	}
	// NotAfter should be ~24h from now (allow 5s tolerance)
	expected := time.Now().Add(24 * time.Hour)
	if diff := tmpl.NotAfter.Sub(expected); diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("NotAfter off by %v", diff)
	}
	found := false
	for _, eku := range tmpl.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			found = true
		}
	}
	if !found {
		t.Error("missing ClientAuth ExtKeyUsage")
	}
}

func TestSignCSRPEM(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	csrTemplate := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "test"}}
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, csrTemplate, key)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	t.Run("valid CSR", func(t *testing.T) {
		certPEM, err := ca.SignCSRPEM(csrPEM, 24*time.Hour)
		if err != nil {
			t.Fatalf("SignCSRPEM failed: %v", err)
		}
		block, _ := pem.Decode(certPEM)
		if block == nil {
			t.Fatal("returned PEM is not decodable")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("returned cert is not parseable: %v", err)
		}
		if cert.Subject.CommonName != "test" {
			t.Errorf("CN = %q, want %q", cert.Subject.CommonName, "test")
		}
		if err := cert.CheckSignatureFrom(ca.GetCACertificate()); err != nil {
			t.Errorf("signature check failed: %v", err)
		}
	})

	t.Run("garbage bytes", func(t *testing.T) {
		_, err := ca.SignCSRPEM([]byte("not pem at all"), 24*time.Hour)
		if err == nil {
			t.Error("expected error for garbage input")
		}
	})

	t.Run("valid PEM bad DER", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("bad der")})
		_, err := ca.SignCSRPEM(badPEM, 24*time.Hour)
		if err == nil {
			t.Error("expected error for bad DER inside PEM")
		}
	})
}

func TestGetCACertificateServiceWrapper(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	svc := certs.NewCertificateService(ca, nil)
	cert, err := svc.GetCACertificate()
	if err != nil {
		t.Fatalf("GetCACertificate failed: %v", err)
	}
	if cert != ca.GetCACertificate() {
		t.Error("service should return same cert pointer as CAManager")
	}
}

func TestLoadCAErrorPaths(t *testing.T) {
	dir := t.TempDir()
	// Generate a valid CA to get valid PEM files
	validCA, err := certs.GenerateCA(dir+"/valid.crt", dir+"/valid.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}
	_ = validCA

	validCertPEM, _ := os.ReadFile(dir + "/valid.crt")
	validKeyPEM, _ := os.ReadFile(dir + "/valid.key")

	t.Run("garbage cert file", func(t *testing.T) {
		d := t.TempDir()
		os.WriteFile(d+"/ca.crt", []byte("garbage"), 0644)
		os.WriteFile(d+"/ca.key", validKeyPEM, 0644)
		_, err := certs.NewCAManager(d+"/ca.crt", d+"/ca.key")
		if err == nil {
			t.Error("expected error for garbage cert file")
		}
	})

	t.Run("valid PEM bad DER cert", func(t *testing.T) {
		d := t.TempDir()
		badCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("bad der")})
		os.WriteFile(d+"/ca.crt", badCert, 0644)
		os.WriteFile(d+"/ca.key", validKeyPEM, 0644)
		_, err := certs.NewCAManager(d+"/ca.crt", d+"/ca.key")
		if err == nil {
			t.Error("expected error for bad DER in cert PEM")
		}
	})

	t.Run("valid cert garbage key", func(t *testing.T) {
		d := t.TempDir()
		os.WriteFile(d+"/ca.crt", validCertPEM, 0644)
		os.WriteFile(d+"/ca.key", []byte("garbage"), 0644)
		_, err := certs.NewCAManager(d+"/ca.crt", d+"/ca.key")
		if err == nil {
			t.Error("expected error for garbage key file")
		}
	})

	t.Run("valid cert valid PEM bad DER key", func(t *testing.T) {
		d := t.TempDir()
		os.WriteFile(d+"/ca.crt", validCertPEM, 0644)
		badKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("bad der")})
		os.WriteFile(d+"/ca.key", badKey, 0644)
		_, err := certs.NewCAManager(d+"/ca.crt", d+"/ca.key")
		if err == nil {
			t.Error("expected error for bad DER in key PEM")
		}
	})
}

func TestNewCAManagerFromPEM_BadDER(t *testing.T) {
	dir := t.TempDir()
	_, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}
	validCertPEM, _ := os.ReadFile(dir + "/ca.crt")
	validKeyPEM, _ := os.ReadFile(dir + "/ca.key")

	t.Run("invalid cert DER", func(t *testing.T) {
		badCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("bad")})
		_, err := certs.NewCAManagerFromPEM(badCert, validKeyPEM)
		if err == nil {
			t.Error("expected error for invalid cert DER")
		}
	})

	t.Run("invalid key DER", func(t *testing.T) {
		badKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("bad")})
		_, err := certs.NewCAManagerFromPEM(validCertPEM, badKey)
		if err == nil {
			t.Error("expected error for invalid key DER")
		}
	})
}

func TestGenerateCA_KeyExistsOnly(t *testing.T) {
	dir := t.TempDir()
	// Create only the key file
	os.WriteFile(dir+"/ca.key", []byte("existing key"), 0644)
	_, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err == nil {
		t.Error("expected error when key file already exists")
	}
}

func TestSignCSR_BadSignature(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	csrTemplate := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "tampered"}}
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, csrTemplate, key)

	// Parse the CSR, tamper with its signature, then call SignCSR
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		t.Fatalf("failed to parse CSR: %v", err)
	}
	// Corrupt the signature by flipping bits
	csr.Signature[0] ^= 0xFF

	_, err = ca.SignCSR(csr, 24*time.Hour)
	if err == nil {
		t.Error("expected error for tampered CSR signature")
	}
}

func TestSignRawCSR_BadDER(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	_, err = ca.SignRawCSR([]byte("garbage der"), 24*time.Hour)
	if err == nil {
		t.Error("expected error for garbage DER input")
	}
}

func TestGenerateCRL(t *testing.T) {
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}
	_ = ca

	crlPath := dir + "/ca.crl"
	// CRL should already exist from GenerateCA
	data, err := os.ReadFile(crlPath)
	if err != nil {
		t.Fatalf("CRL file not found: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("CRL is not valid PEM")
	}
	if block.Type != "X509 CRL" {
		t.Errorf("PEM type = %q, want %q", block.Type, "X509 CRL")
	}

	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CRL: %v", err)
	}
	if err := crl.CheckSignatureFrom(ca.GetCACertificate()); err != nil {
		t.Errorf("CRL signature check failed: %v", err)
	}
}
