package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CAManager manages the Certificate Authority for signing device certificates.
// It handles CA certificate generation, loading, and device certificate signing.
type CAManager struct {
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certPath  string
	keyPath   string
}

// NewCAManager loads an existing CA from the specified file paths.
// Returns an error if the files don't exist or can't be parsed.
// Use GenerateCA() to create new CA files before calling this.
func NewCAManager(certPath, keyPath string) (*CAManager, error) {
	manager := &CAManager{
		certPath: certPath,
		keyPath:  keyPath,
	}
	if err := manager.loadCA(); err != nil {
		return nil, fmt.Errorf("failed to load CA (use 'localmdm-cli certs init' to generate): %w", err)
	}
	// Ensure CRL exists alongside the CA cert
	crlPath := filepath.Join(filepath.Dir(certPath), "ca.crl")
	if _, err := os.Stat(crlPath); os.IsNotExist(err) {
		if crlErr := manager.GenerateCRL(); crlErr != nil {
			slog.Warn("failed to auto-generate CRL on startup", "error", crlErr, "path", crlPath)
		}
	}
	return manager, nil
}

// GenerateCA creates a new CA certificate and key at the specified paths.
// Returns an error if files already exist (won't overwrite).
func GenerateCA(certPath, keyPath string) (*CAManager, error) {
	if _, err := os.Stat(certPath); err == nil {
		return nil, fmt.Errorf("CA certificate already exists at %s", certPath)
	}
	if _, err := os.Stat(keyPath); err == nil {
		return nil, fmt.Errorf("CA key already exists at %s", keyPath)
	}
	manager := &CAManager{certPath: certPath, keyPath: keyPath}
	if err := manager.generateCA(); err != nil {
		return nil, fmt.Errorf("failed to generate CA: %w", err)
	}
	return manager, nil
}

// NewCAManagerFromPEM creates a CA manager from PEM-encoded certificate and key bytes.
// Use this in production where secrets come from environment variables (AWS Secrets Manager/SSM)
// rather than filesystem paths.
func NewCAManagerFromPEM(certPEM, keyPEM []byte) (*CAManager, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode CA key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	return &CAManager{caCert: cert, caKey: key}, nil
}

func (m *CAManager) loadCA() error {
	// Load certificate
	certPEM, err := os.ReadFile(m.certPath)
	if err != nil {
		return err
	}
	
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	
	// Load private key
	keyPEM, err := os.ReadFile(m.keyPath)
	if err != nil {
		return err
	}
	
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode key PEM")
	}
	
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}
	
	m.caCert = cert
	m.caKey = key
	
	return nil
}

func (m *CAManager) generateCA() error {
	// Generate RSA key
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	
	// Create certificate template
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Local MDM"},
			CommonName:   "Local MDM Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	
	// Self-sign
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}
	
	// Save to disk
	if err := m.saveCertificate(cert, key); err != nil {
		return err
	}
	
	m.caCert = cert
	m.caKey = key
	
	// Generate an empty CRL alongside the CA
	if err := m.GenerateCRL(); err != nil {
		return fmt.Errorf("generate initial CRL: %w", err)
	}
	
	return nil
}

func (m *CAManager) saveCertificate(cert *x509.Certificate, key *rsa.PrivateKey) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.certPath), 0700); err != nil {
		return err
	}
	
	// Save certificate
	certFile, err := os.Create(m.certPath)
	if err != nil {
		return err
	}
	defer certFile.Close()
	
	if err := pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}); err != nil {
		return err
	}
	
	// Save private key
	keyFile, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyFile.Close()
	
	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		return err
	}
	
	return nil
}

// GenerateCRL creates an empty CRL signed by this CA and writes it next to the CA cert.
// The CRL file is placed at the same directory as the CA cert with the name "ca.crl".
func (m *CAManager) GenerateCRL() error {
	if m.certPath == "" {
		return nil // PEM-only manager (production), no filesystem path
	}
	template := &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now(),
		NextUpdate: time.Now().AddDate(0, 3, 0), // 3 months
	}
	crlDER, err := x509.CreateRevocationList(rand.Reader, template, m.caCert, m.caKey)
	if err != nil {
		return fmt.Errorf("create revocation list: %w", err)
	}
	crlPath := filepath.Join(filepath.Dir(m.certPath), "ca.crl")
	return os.WriteFile(crlPath, pem.EncodeToMemory(&pem.Block{
		Type:  "X509 CRL",
		Bytes: crlDER,
	}), 0644)
}

func (m *CAManager) GetCACertificate() *x509.Certificate {
	return m.caCert
}

// GetCAPrivateKey returns the CA private key (needed for SCEP envelope decryption).
func (m *CAManager) GetCAPrivateKey() *rsa.PrivateKey {
	return m.caKey
}

// CertTemplate returns a certificate template for SCEP signing (serial, validity, key usage).
func (m *CAManager) CertTemplate(validity time.Duration) *x509.Certificate {
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
}

func (m *CAManager) GetCACertificatePEM() ([]byte, error) {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: m.caCert.Raw,
	}), nil
}

func (m *CAManager) SignCSR(csr *x509.CertificateRequest, validity time.Duration) (*x509.Certificate, error) {
	// Verify CSR signature
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid CSR signature: %w", err)
	}
	
	// Generate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	
	// Create certificate template from CSR
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	
	// Sign certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, m.caCert, csr.PublicKey, m.caKey)
	if err != nil {
		return nil, err
	}
	
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}
	
	return cert, nil
}

// SignRawCSR signs a DER-encoded CSR without parsing the subject (for Windows
// CSRs that contain non-PrintableString characters Go's parser rejects).
func (m *CAManager) SignRawCSR(csrDER []byte, validity time.Duration) (*x509.Certificate, error) {
	// Extract public key and raw subject from CSR
	pubKey, rawSubject, err := parseCSRFields(csrDER)
	if err != nil {
		return nil, fmt.Errorf("extract public key from CSR: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Preserve original subject if extracted, otherwise fall back
	if len(rawSubject) > 0 {
		template.RawSubject = rawSubject
	} else {
		template.Subject = pkix.Name{CommonName: "MDMDeviceCert"}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, m.caCert, pubKey, m.caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDER)
}

// parseCSRFields extracts the public key and raw subject bytes from a DER-encoded CSR.
// It tries standard parsing first; on failure it falls back to manual ASN.1 parsing
// to handle CSRs with non-PrintableString subjects that Go's parser rejects.
func parseCSRFields(csrDER []byte) (interface{}, []byte, error) {
	// Try standard parsing first (works for most CSRs)
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err == nil {
		return csr.PublicKey, csr.RawSubject, nil
	}
	// Fallback: parse the SubjectPublicKeyInfo from the raw ASN.1
	// CSR structure: SEQUENCE { CertificationRequestInfo, SignatureAlgorithm, Signature }
	// CertificationRequestInfo: SEQUENCE { Version, Subject, SubjectPublicKeyInfo, ... }
	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(csrDER, &raw)
	if err != nil || len(rest) > 0 {
		return nil, nil, fmt.Errorf("unmarshal CSR outer: %w", err)
	}
	// Parse inner SEQUENCE (CertificationRequestInfo)
	var inner asn1.RawValue
	rest2, err := asn1.Unmarshal(raw.Bytes, &inner)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal CertReqInfo: %w", err)
	}
	_ = rest2
	// Skip version
	var version asn1.RawValue
	remaining, err := asn1.Unmarshal(inner.Bytes, &version)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal version: %w", err)
	}
	// Extract subject (raw bytes including tag+length)
	var subject asn1.RawValue
	remaining, err = asn1.Unmarshal(remaining, &subject)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal subject: %w", err)
	}
	// Parse SubjectPublicKeyInfo
	var spki asn1.RawValue
	_, err = asn1.Unmarshal(remaining, &spki)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal SPKI: %w", err)
	}
	// Re-parse as x509 public key
	pub, err := x509.ParsePKIXPublicKey(spki.FullBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse public key: %w", err)
	}
	return pub, subject.FullBytes, nil
}

func (m *CAManager) SignCSRPEM(csrPEM []byte, validity time.Duration) ([]byte, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode CSR PEM")
	}
	
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	cert, err := m.SignCSR(csr, validity)
	if err != nil {
		return nil, err
	}
	
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}), nil
}
