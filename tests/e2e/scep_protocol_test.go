package e2e

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mozilla.org/pkcs7"
)

// TestE2E_SCEP_FullProtocolFlow exercises the complete SCEP protocol
// against a real SCEP handler backed by PostgreSQL challenge storage:
//
// 1. GetCACert → PKCS#7 degenerate envelope containing CA cert
// 2. GetCACaps → protocol capabilities
// 3. PKIOperation → CSR with challenge → PKCS#7 signed cert
// 4. Certificate chain verification (signed cert → CA)
// 5. Challenge replay protection
//
// This is the openssl-equivalent verification: every response is parsed
// as PKCS#7 DER, the same format openssl pkcs7/smime commands expect.
func TestE2E_SCEP_FullProtocolFlow(t *testing.T) {
	database := setupDB(t)
	defer database.Close()

	dir := t.TempDir()
	ca, err := certs.NewCAManager(dir+"/ca.crt", dir+"/ca.key")
	require.NoError(t, err)

	challengeMgr := scep.NewChallengeManager(database.Writer)
	handler := scep.NewHandler(ca, challengeMgr, slog.Default())
	server := httptest.NewServer(handler)
	defer server.Close()

	// === Step 1: GetCACert — PKCS#7 degenerate envelope ===
	resp, err := http.Get(server.URL + "?operation=GetCACert")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/x-x509-ca-ra-cert", resp.Header.Get("Content-Type"))

	caCertBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Parse as PKCS#7 (equivalent to: openssl pkcs7 -inform DER -print_certs)
	p7, err := pkcs7.Parse(caCertBody)
	require.NoError(t, err, "GetCACert must return valid PKCS#7")
	require.NotEmpty(t, p7.Certificates, "PKCS#7 must contain CA certificate")

	caCert := p7.Certificates[0]
	assert.True(t, caCert.IsCA)
	assert.Equal(t, "Local MDM Root CA", caCert.Subject.CommonName)
	t.Logf("✓ GetCACert: CN=%s, Serial=%s", caCert.Subject.CommonName, caCert.SerialNumber)

	// === Step 2: GetCACaps ===
	resp2, err := http.Get(server.URL + "?operation=GetCACaps")
	require.NoError(t, err)
	defer resp2.Body.Close()
	capsBody, _ := io.ReadAll(resp2.Body)
	assert.Contains(t, string(capsBody), "POSTPKIOperation")
	assert.Contains(t, string(capsBody), "SHA-256")
	t.Logf("✓ GetCACaps: %s", string(capsBody))

	// === Step 3: Generate challenge, build CSR, submit PKIOperation ===
	challenge, err := challengeMgr.GenerateChallenge("scep-e2e-device", 5*time.Minute)
	require.NoError(t, err)

	csrDER := mustBuildCSRWithChallengeE2E(t, challenge, "scep-e2e-device")

	resp3, err := http.Post(server.URL+"?operation=PKIOperation", "application/octet-stream", bytes.NewReader(csrDER))
	require.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	assert.Equal(t, "application/x-pki-message", resp3.Header.Get("Content-Type"))

	certBody, err := io.ReadAll(resp3.Body)
	require.NoError(t, err)

	// Parse response as PKCS#7 (equivalent to: openssl pkcs7 -inform DER -print_certs)
	p7Cert, err := pkcs7.Parse(certBody)
	require.NoError(t, err, "PKIOperation response must be valid PKCS#7")
	require.NotEmpty(t, p7Cert.Certificates)

	signedCert := p7Cert.Certificates[0]
	assert.False(t, signedCert.IsCA)
	assert.Equal(t, "scep-e2e-device", signedCert.Subject.CommonName)
	t.Logf("✓ PKIOperation: CN=%s, Serial=%s, Issuer=%s",
		signedCert.Subject.CommonName, signedCert.SerialNumber, signedCert.Issuer.CommonName)

	// === Step 4: Certificate chain verification ===
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	_, err = signedCert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "signed cert must chain to CA")
	t.Log("✓ Chain verification: signed cert → CA (ExtKeyUsage=ClientAuth)")

	// === Step 5: Challenge replay protection ===
	resp4, err := http.Post(server.URL+"?operation=PKIOperation", "application/octet-stream", bytes.NewReader(csrDER))
	require.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp4.StatusCode, "replayed challenge must be rejected")
	t.Log("✓ Replay protection: second use of challenge rejected")
}

// mustBuildCSRWithChallengeE2E builds a DER-encoded PKCS#10 CSR with challengePassword.
func mustBuildCSRWithChallengeE2E(t *testing.T, password, cn string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	version, _ := asn1.Marshal(0)
	subject, _ := asn1.Marshal(pkix.RDNSequence{
		pkix.RelativeDistinguishedNameSET{
			{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: cn},
		},
	})
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	challengeOID, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 7})
	pwVal, _ := asn1.Marshal(password)
	valSet, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSet, IsCompound: true, Bytes: pwVal})
	attrSeq, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: append(challengeOID, valSet...)})
	attrs, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: attrSeq})

	criContent := catBytes(version, subject, spki, attrs)
	cri, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: criContent})

	h := sha256.Sum256(cri)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	require.NoError(t, err)

	algOID, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	algNull, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagNull})
	sigAlg, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: append(algOID, algNull...)})
	sigBits, _ := asn1.Marshal(asn1.BitString{Bytes: sig, BitLength: len(sig) * 8})

	csrDER, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: catBytes(cri, sigAlg, sigBits)})
	return csrDER
}

func catBytes(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
