package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type CAManager struct {
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certPath  string
	keyPath   string
}

func NewCAManager(certPath, keyPath string) (*CAManager, error) {
	manager := &CAManager{
		certPath: certPath,
		keyPath:  keyPath,
	}
	
	// Try to load existing CA
	if err := manager.loadCA(); err != nil {
		// Generate new CA if doesn't exist
		if err := manager.generateCA(); err != nil {
			return nil, fmt.Errorf("failed to generate CA: %w", err)
		}
	}
	
	return manager, nil
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
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
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

func (m *CAManager) GetCACertificate() *x509.Certificate {
	return m.caCert
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
