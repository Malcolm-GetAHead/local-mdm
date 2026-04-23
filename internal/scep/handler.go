package scep

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/certs"
	sceplib "github.com/smallstep/scep"
	"go.mozilla.org/pkcs7"
)

// Handler serves SCEP protocol requests (GetCACert and PKCSReq).
type Handler struct {
	ca      *certs.CAManager
	store   ChallengeStore
	logger  *slog.Logger
	certTTL time.Duration
}

// NewHandler creates a SCEP HTTP handler.
func NewHandler(ca *certs.CAManager, store ChallengeStore, logger *slog.Logger) *Handler {
	return &Handler{ca: ca, store: store, logger: logger, certTTL: 365 * 24 * time.Hour}
}

// ServeHTTP handles GET (GetCACert/GetCACaps) and POST (PKIOperation) SCEP operations.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	op := r.URL.Query().Get("operation")

	switch r.Method {
	case http.MethodGet:
		switch op {
		case "GetCACaps":
			h.getCACaps(w)
		default:
			h.getCACert(w)
		}
	case http.MethodPost:
		if op == "PKIOperation" {
			h.pkiOperation(w, r)
		} else {
			http.Error(w, "unsupported operation", http.StatusBadRequest)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getCACert(w http.ResponseWriter) {
	cert := h.ca.GetCACertificate()
	if cert == nil {
		http.Error(w, "CA not available", http.StatusServiceUnavailable)
		return
	}
	// Return CA cert in PKCS#7 degenerate certificates-only envelope
	// This is the format real SCEP clients (including Apple's) expect
	degenerateData, err := pkcs7.DegenerateCertificate(cert.Raw)
	if err != nil {
		h.logger.Error("failed to create PKCS#7 degenerate cert", "error", err)
		// Fall back to raw DER
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Write(cert.Raw)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-ra-cert")
	w.Write(degenerateData)
}

func (h *Handler) getCACaps(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "POSTPKIOperation\nSHA-256\nAES\nSCEPStandard\n")
}

func (h *Handler) pkiOperation(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}

	caCert := h.ca.GetCACertificate()
	caKey := h.ca.GetCAPrivateKey()

	// Try full SCEP protocol (PKCS#7 SignedData + EnvelopedData) first
	msg, err := sceplib.ParsePKIMessage(body)
	if err == nil {
		if err := msg.DecryptPKIEnvelope(caCert, caKey); err != nil {
			h.logger.Warn("failed to decrypt SCEP envelope", "error", err)
			http.Error(w, "failed to decrypt envelope", http.StatusBadRequest)
			return
		}
		if msg.CSRReqMessage == nil || msg.CSRReqMessage.CSR == nil {
			http.Error(w, "no CSR in SCEP message", http.StatusBadRequest)
			return
		}
		csr := msg.CSRReqMessage.CSR
		pw := msg.CSRReqMessage.ChallengePassword
		if pw == "" {
			pw = extractChallengePassword(csr)
		}
		if pw == "" {
			http.Error(w, "missing challenge password", http.StatusForbidden)
			return
		}
		deviceID, valid := h.store.ValidateChallenge(pw)
		if !valid {
			h.logger.Warn("SCEP challenge validation failed")
			certRep, _ := msg.Fail(caCert, caKey, sceplib.BadRequest)
			w.Header().Set("Content-Type", "application/x-pki-message")
			w.Write(certRep.Raw)
			return
		}
		cert, err := h.ca.SignCSR(csr, h.certTTL)
		if err != nil {
			h.logger.Error("failed to sign CSR", "error", err, "device_id", deviceID)
			certRep, _ := msg.Fail(caCert, caKey, sceplib.BadRequest)
			w.Header().Set("Content-Type", "application/x-pki-message")
			w.Write(certRep.Raw)
			return
		}
		h.logger.Info("SCEP certificate issued", "device_id", deviceID, "serial", cert.SerialNumber.String())
		certRep, err := msg.Success(caCert, caKey, cert)
		if err != nil {
			h.logger.Error("failed to build SCEP success response", "error", err)
			http.Error(w, "response generation failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-pki-message")
		w.Write(certRep.Raw)
		return
	}

	// Fall back to raw/base64 CSR (for simple testing without full SCEP envelope)
	csr, err := parseCSR(body)
	if err != nil {
		h.logger.Warn("failed to parse SCEP request", "error", err)
		http.Error(w, "invalid CSR", http.StatusBadRequest)
		return
	}

	pw := extractChallengePassword(csr)
	if pw == "" {
		http.Error(w, "missing challenge password", http.StatusForbidden)
		return
	}

	deviceID, valid := h.store.ValidateChallenge(pw)
	if !valid {
		h.logger.Warn("SCEP challenge validation failed")
		http.Error(w, "invalid or expired challenge", http.StatusForbidden)
		return
	}

	cert, err := h.ca.SignCSR(csr, h.certTTL)
	if err != nil {
		h.logger.Error("failed to sign CSR", "error", err, "device_id", deviceID)
		http.Error(w, "signing failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("SCEP certificate issued", "device_id", deviceID, "serial", cert.SerialNumber.String())

	// Wrap signed cert in PKCS#7 degenerate certificates-only envelope
	respData, err := buildCertResponse(cert, caCert)
	if err != nil {
		h.logger.Error("failed to build PKCS#7 response", "error", err)
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Write(cert.Raw)
		return
	}
	w.Header().Set("Content-Type", "application/x-pki-message")
	w.Write(respData)
}

// buildCertResponse wraps the signed cert (and CA cert) in a PKCS#7 degenerate envelope
func buildCertResponse(cert, caCert *x509.Certificate) ([]byte, error) {
	certs := cert.Raw
	if caCert != nil {
		// Include both signed cert and CA cert in the chain
		certs = append(cert.Raw, caCert.Raw...)
	}
	// Use degenerate certificate for the signed cert
	return pkcs7.DegenerateCertificate(certs)
}

var challengePasswordOID = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 7}

func parseCSR(data []byte) (*x509.CertificateRequest, error) {
	if csr, err := x509.ParseCertificateRequest(data); err == nil {
		return csr, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(data)); err == nil {
		if csr, err := x509.ParseCertificateRequest(decoded); err == nil {
			return csr, nil
		}
	}
	return nil, fmt.Errorf("unable to parse CSR from request body")
}

// extractChallengePassword extracts the challengePassword from CSR attributes.
// Go's x509.ParseCertificateRequest doesn't reliably populate the deprecated
// Attributes field, so we also parse the raw TBS bytes.
func extractChallengePassword(csr *x509.CertificateRequest) string {
	// Try the parsed Attributes first
	for _, attr := range csr.Attributes {
		if attr.Type.Equal(challengePasswordOID) {
			for _, valSet := range attr.Value {
				for _, val := range valSet {
					if s, ok := val.Value.(string); ok {
						return s
					}
				}
			}
		}
	}
	// Fall back to parsing from raw TBS bytes
	return extractChallengeFromRawTBS(csr.RawTBSCertificateRequest)
}

func extractChallengeFromRawTBS(raw []byte) string {
	var tbs struct {
		Version int
		Subject asn1.RawValue
		SPKI    asn1.RawValue
		Attrs   []asn1.RawValue `asn1:"tag:0,set,optional"`
	}
	if _, err := asn1.Unmarshal(raw, &tbs); err != nil {
		return ""
	}
	for _, rawAttr := range tbs.Attrs {
		var attr struct {
			Type  asn1.ObjectIdentifier
			Value asn1.RawValue `asn1:"set"`
		}
		data := rawAttr.FullBytes
		if len(data) == 0 {
			data = rawAttr.Bytes
		}
		if _, err := asn1.Unmarshal(data, &attr); err != nil {
			continue
		}
		if attr.Type.Equal(challengePasswordOID) {
			var s string
			if _, err := asn1.Unmarshal(attr.Value.Bytes, &s); err == nil {
				return s
			}
		}
	}
	return ""
}
