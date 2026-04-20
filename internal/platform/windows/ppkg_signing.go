package windows

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// PPKGSigner signs provisioning packages with a code signing certificate.
type PPKGSigner struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

// NewPPKGSigner loads a signing certificate and key from PEM files.
// If the files don't exist and generate is true, creates a self-signed
// code signing certificate for development/testing.
func NewPPKGSigner(certPath, keyPath string, generate bool) (*PPKGSigner, error) {
	cert, key, err := loadSigningCert(certPath, keyPath)
	if err != nil && generate {
		cert, key, err = generateSigningCert(certPath, keyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load signing certificate: %w", err)
	}
	return &PPKGSigner{cert: cert, key: key}, nil
}

// Sign creates a detached SHA-256/RSA signature over the ppkg data.
// Returns the signature bytes (DER-encoded PKCS#1 v1.5).
func (s *PPKGSigner) Sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign ppkg: %w", err)
	}
	return sig, nil
}

// CertificatePEM returns the signing certificate in PEM format.
func (s *PPKGSigner) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.cert.Raw})
}

func loadSigningCert(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

// generateSigningCert creates a self-signed code signing certificate for dev/testing.
// Production deployments should replace these files with a real code signing certificate.
func generateSigningCert(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Local MDM"},
			CommonName:   "Local MDM Code Signing (Development)",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	// Save to disk
	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0644); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}
