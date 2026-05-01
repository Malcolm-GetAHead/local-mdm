package scep

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/certs"
	sceplib "github.com/micromdm/scep/scep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mozilla.org/pkcs7"
)

type mockStore struct {
	challenges map[string]string
}

func (m *mockStore) GenerateChallenge(_ context.Context, deviceID string, ttl time.Duration) (string, error) {
	pw := "test-pw"
	m.challenges[pw] = deviceID
	return pw, nil
}
func (m *mockStore) ValidateChallenge(_ context.Context, password string) (string, bool) {
	id, ok := m.challenges[password]
	if ok {
		delete(m.challenges, password)
	}
	return id, ok
}
func (m *mockStore) CleanupExpired(_ context.Context) {}

func setupTestCA(t *testing.T) *certs.CAManager {
	t.Helper()
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	require.NoError(t, err)
	return ca
}

func TestHandler_GetCACert(t *testing.T) {
	ca := setupTestCA(t)
	h := NewHandler(ca, &mockStore{challenges: map[string]string{}}, slog.Default())

	req := httptest.NewRequest("GET", "/scep?operation=GetCACert", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-x509-ca-ra-cert", w.Header().Get("Content-Type"))
	p7, err := pkcs7.Parse(w.Body.Bytes())
	require.NoError(t, err)
	require.NotEmpty(t, p7.Certificates)
	assert.Equal(t, "Local MDM Root CA", p7.Certificates[0].Subject.CommonName)
}

func TestHandler_GetCACaps(t *testing.T) {
	ca := setupTestCA(t)
	h := NewHandler(ca, &mockStore{challenges: map[string]string{}}, slog.Default())

	req := httptest.NewRequest("GET", "/scep?operation=GetCACaps", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "POSTPKIOperation")
	assert.Contains(t, w.Body.String(), "SHA-256")
}

func TestHandler_DefaultGET(t *testing.T) {
	ca := setupTestCA(t)
	h := NewHandler(ca, &mockStore{challenges: map[string]string{}}, slog.Default())

	req := httptest.NewRequest("GET", "/scep", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-x509-ca-ra-cert", w.Header().Get("Content-Type"))
}

func TestHandler_PKIOperation(t *testing.T) {
	ca := setupTestCA(t)
	store := &mockStore{challenges: map[string]string{"valid-pw": "device-1"}}
	h := NewHandler(ca, store, slog.Default())

	t.Run("signs CSR with valid challenge", func(t *testing.T) {
		csrDER := mustBuildCSRWithChallenge(t, "valid-pw")
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(csrDER))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Response is PKCS#7 degenerate certificate envelope
		p7, err := pkcs7.Parse(w.Body.Bytes())
		require.NoError(t, err)
		require.NotEmpty(t, p7.Certificates)
		cert := p7.Certificates[0]
		assert.False(t, cert.IsCA)
		assert.Equal(t, "test-device", cert.Subject.CommonName)
	})

	t.Run("rejects invalid challenge", func(t *testing.T) {
		csrDER := mustBuildCSRWithChallenge(t, "bad-pw")
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(csrDER))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("rejects CSR without challenge", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
			Subject: pkix.Name{CommonName: "test"},
		}, key)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(csrDER))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("rejects invalid body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader([]byte("garbage")))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	ca := setupTestCA(t)
	h := NewHandler(ca, &mockStore{challenges: map[string]string{}}, slog.Default())

	req := httptest.NewRequest("DELETE", "/scep", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandler_UnsupportedPOSTOperation(t *testing.T) {
	ca := setupTestCA(t)
	h := NewHandler(ca, &mockStore{challenges: map[string]string{}}, slog.Default())

	req := httptest.NewRequest("POST", "/scep?operation=GetCACert", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// mustBuildCSRWithChallenge builds a DER-encoded PKCS#10 CSR with a challengePassword
// attribute. Go's x509 package doesn't support creating CSRs with attributes, so we
// construct the ASN.1 manually per RFC 2986.
func mustBuildCSRWithChallenge(t *testing.T, password string) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// === Build CertificationRequestInfo ===
	// version INTEGER (0)
	version, _ := asn1.Marshal(0)

	// subject Name (RDNSequence)
	subject, _ := asn1.Marshal(pkix.RDNSequence{
		pkix.RelativeDistinguishedNameSET{
			{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "test-device"},
		},
	})

	// subjectPKInfo SubjectPublicKeyInfo
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	// attributes [0] IMPLICIT SET OF Attribute
	// Attribute ::= SEQUENCE { type OID, values SET OF ANY }
	challengeOID, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 7})
	// The value is a SET containing a single UTF8String
	pwVal, _ := asn1.Marshal(password)
	valSet, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSet, IsCompound: true, Bytes: pwVal})
	attrSeq, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: append(challengeOID, valSet...)})

	// Wrap in context-specific [0] IMPLICIT
	attrs, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: attrSeq})

	// CertificationRequestInfo ::= SEQUENCE { version, subject, spki, attributes }
	criContent := cat(version, subject, spki, attrs)
	cri, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: criContent})

	// === Sign ===
	h := sha256.Sum256(cri)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	require.NoError(t, err)

	// signatureAlgorithm AlgorithmIdentifier (sha256WithRSA)
	algOID, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11})
	algNull, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagNull})
	sigAlg, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: append(algOID, algNull...)})

	// signature BIT STRING
	sigBits, _ := asn1.Marshal(asn1.BitString{Bytes: sig, BitLength: len(sig) * 8})

	// === CertificationRequest ::= SEQUENCE { cri, sigAlg, sig } ===
	csrDER, _ := asn1.Marshal(asn1.RawValue{Class: 0, Tag: asn1.TagSequence, IsCompound: true, Bytes: cat(cri, sigAlg, sigBits)})

	// Verify it parses and has the attribute
	parsed, err := x509.ParseCertificateRequest(csrDER)
	require.NoError(t, err, "CSR must parse")

	// Verify challenge password is extractable
	pw := extractChallengePassword(parsed)
	require.Equal(t, password, pw, "challenge password must be extractable from CSR")

	return csrDER
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestHandler_PKIOperation_Base64CSR(t *testing.T) {
	ca := setupTestCA(t)
	store := &mockStore{challenges: map[string]string{"b64-pw": "device-b64"}}
	h := NewHandler(ca, store, slog.Default())

	csrDER := mustBuildCSRWithChallenge(t, "b64-pw")
	b64 := []byte(base64.StdEncoding.EncodeToString(csrDER))

	req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(b64))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	p7, err := pkcs7.Parse(w.Body.Bytes())
	require.NoError(t, err)
	require.NotEmpty(t, p7.Certificates)
}

func TestHandler_PKIOperation_SCEPEnvelope(t *testing.T) {
	ca := setupTestCA(t)
	caCert := ca.GetCACertificate()

	t.Run("valid challenge via SCEP protocol", func(t *testing.T) {
		store := &mockStore{challenges: map[string]string{"scep-pw": "device-scep"}}
		h := NewHandler(ca, store, slog.Default())

		envelope := mustBuildSCEPRequest(t, caCert, "scep-pw")
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(envelope))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-pki-message", w.Header().Get("Content-Type"))
	})

	t.Run("invalid challenge via SCEP protocol", func(t *testing.T) {
		store := &mockStore{challenges: map[string]string{}}
		h := NewHandler(ca, store, slog.Default())

		envelope := mustBuildSCEPRequest(t, caCert, "wrong-pw")
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(envelope))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		// SCEP failure response (still 200 with PKCS#7 failure message)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-pki-message", w.Header().Get("Content-Type"))
	})

	t.Run("decrypt error wrong CA", func(t *testing.T) {
		store := &mockStore{challenges: map[string]string{}}
		h := NewHandler(ca, store, slog.Default())

		otherCA := setupTestCA(t)
		envelope := mustBuildSCEPRequest(t, otherCA.GetCACertificate(), "any-pw")
		req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(envelope))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// mustBuildSCEPRequest builds a proper SCEP PKCSReq message using the micromdm/scep library.
func mustBuildSCEPRequest(t *testing.T, recipientCert *x509.Certificate, challengePassword string) []byte {
	t.Helper()

	// Generate device key and self-signed cert
	deviceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	deviceTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "scep-device"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	deviceCertDER, err := x509.CreateCertificate(rand.Reader, deviceTmpl, deviceTmpl, &deviceKey.PublicKey, deviceKey)
	require.NoError(t, err)
	deviceCert, err := x509.ParseCertificate(deviceCertDER)
	require.NoError(t, err)

	// Build CSR with challenge password
	csrDER := mustBuildCSRWithChallenge(t, challengePassword)
	csr, err := x509.ParseCertificateRequest(csrDER)
	require.NoError(t, err)

	// Build SCEP message template
	tmpl := &sceplib.PKIMessage{
		MessageType: sceplib.PKCSReq,
		Recipients:  []*x509.Certificate{recipientCert},
		SignerCert:  deviceCert,
		SignerKey:   deviceKey,
	}

	msg, err := sceplib.NewCSRRequest(csr, tmpl)
	require.NoError(t, err)
	return msg.Raw
}


func TestHandler_PostIssueHook_CalledWithDeviceID(t *testing.T) {
	ca := setupTestCA(t)
	store := &mockStore{challenges: map[string]string{"hook-pw": "ent-123:token:abc456"}}
	h := NewHandler(ca, store, slog.Default())

	var hookCalled bool
	var hookDeviceID string
	h.SetPostIssueHook(func(deviceID string) {
		hookCalled = true
		hookDeviceID = deviceID
	})

	csrDER := mustBuildCSRWithChallenge(t, "hook-pw")
	b64 := []byte(base64.StdEncoding.EncodeToString(csrDER))
	req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(b64))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hookCalled, "PostIssueHook should be called")
	assert.Equal(t, "ent-123:token:abc456", hookDeviceID)
}

func TestHandler_PostIssueHook_NotCalledOnFailure(t *testing.T) {
	ca := setupTestCA(t)
	store := &mockStore{challenges: map[string]string{}} // no valid challenge
	h := NewHandler(ca, store, slog.Default())

	var hookCalled bool
	h.SetPostIssueHook(func(deviceID string) {
		hookCalled = true
	})

	csrDER := mustBuildCSRWithChallenge(t, "bad-pw")
	b64 := []byte(base64.StdEncoding.EncodeToString(csrDER))
	req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(b64))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.False(t, hookCalled, "PostIssueHook should NOT be called on failed challenge")
}

func TestHandler_PostIssueHook_SCEPEnvelope(t *testing.T) {
	ca := setupTestCA(t)
	caCert := ca.GetCACertificate()
	store := &mockStore{challenges: map[string]string{"scep-hook-pw": "ent-456:token:def789"}}
	h := NewHandler(ca, store, slog.Default())

	var hookDeviceID string
	h.SetPostIssueHook(func(deviceID string) {
		hookDeviceID = deviceID
	})

	envelope := mustBuildSCEPRequest(t, caCert, "scep-hook-pw")
	req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(envelope))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ent-456:token:def789", hookDeviceID)
}

func TestHandler_PostIssueHook_NoTokenMetadata(t *testing.T) {
	ca := setupTestCA(t)
	store := &mockStore{challenges: map[string]string{"plain-pw": "just-an-enterprise-id"}}
	h := NewHandler(ca, store, slog.Default())

	var hookDeviceID string
	h.SetPostIssueHook(func(deviceID string) {
		hookDeviceID = deviceID
	})

	csrDER := mustBuildCSRWithChallenge(t, "plain-pw")
	b64 := []byte(base64.StdEncoding.EncodeToString(csrDER))
	req := httptest.NewRequest("POST", "/scep?operation=PKIOperation", bytes.NewReader(b64))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Hook is called but with plain deviceID (no :token: separator)
	assert.Equal(t, "just-an-enterprise-id", hookDeviceID)
}
