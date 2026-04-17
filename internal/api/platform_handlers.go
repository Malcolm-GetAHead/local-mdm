package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
)

// macOS enrollment profile download
func (s *Server) handleMacOSEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	enterpriseID, err := parseUUIDParam(r, "enterprise_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	// Verify enterprise exists
	if _, err := s.enterpriseRepo.GetByID(r.Context(), enterpriseID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to verify enterprise", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to verify enterprise")
		return
	}

	// Generate unique SCEP challenge with configurable expiration
	challengeTTL := s.config.Certificates.SCEPChallengeTTL
	if challengeTTL == 0 {
		challengeTTL = 5 * time.Minute
	}
	challenge, err := s.challengeManager.GenerateChallenge(enterpriseID.String(), challengeTTL)
	if err != nil {
		s.logger.Error("failed to generate SCEP challenge", "error", err)
		respondError(w, r, http.StatusInternalServerError, "challenge_generation_failed", "Failed to generate enrollment challenge")
		return
	}

	serverURL := fmt.Sprintf("https://%s", r.Host)
	scepURL := serverURL + "/scep"
	topic := s.config.MacOS.PushTopic
	orgName := "Local MDM"

	// Load CA cert if available
	var caCert *x509.Certificate
	if s.certService != nil {
		caCert, _ = s.certService.GetCACertificate()
	}

	profile, err := macos.GenerateEnrollmentProfile(
		enterpriseID, serverURL, scepURL, topic, challenge, orgName, caCert,
	)
	if err != nil {
		s.logger.Error("failed to generate enrollment profile", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate profile")
		return
	}

	s.logAudit(r, "enrollment.macos.profile_generated", "enterprise", enterpriseID, map[string]interface{}{
		"platform": models.PlatformMacOS,
	})
	if s.metrics != nil {
		s.metrics.EnrollmentsTotal.WithLabelValues(models.PlatformMacOS, "profile_generated").Inc()
	}

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", "attachment; filename=enrollment.mobileconfig")
	w.WriteHeader(http.StatusOK)
	w.Write(profile)
}

// Windows discovery service
func (s *Server) handleWindowsDiscoveryService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Error("failed to read discovery request", "error", err)
		respondError(w, r, http.StatusBadRequest, "read_failed", "Failed to read request")
		return
	}

	if len(body) == 0 {
		respondError(w, r, http.StatusBadRequest, "empty_body", "Request body is empty")
		return
	}

	req, err := windows.ParseDiscoverRequest(body)
	if err != nil {
		s.logger.Error("failed to parse discovery request", "error", err)
		respondError(w, r, http.StatusBadRequest, "parse_failed", "Invalid discovery request")
		return
	}

	s.logger.Info("windows discovery request",
		"email", req.Request.EmailAddress,
		"device_type", req.Request.DeviceType,
	)

	serverURL := fmt.Sprintf("https://%s", r.Host)
	enrollmentURL := serverURL + "/EnrollmentServer/Enrollment.svc"
	policyURL := serverURL + "/EnrollmentServer/Policy.svc"

	resp, err := windows.GenerateDiscoverResponse(enrollmentURL, policyURL)
	if err != nil {
		s.logger.Error("failed to generate discovery response", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate response")
		return
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Windows enrollment service — signs CSR and creates device record
func (s *Server) handleWindowsEnrollmentService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Error("failed to read enrollment request", "error", err)
		respondError(w, r, http.StatusBadRequest, "read_failed", "Failed to read request")
		return
	}

	if len(body) == 0 {
		respondError(w, r, http.StatusBadRequest, "empty_body", "Request body is empty")
		return
	}

	env, err := windows.ParseEnrollmentRequest(body)
	if err != nil {
		s.logger.Error("failed to parse enrollment request", "error", err)
		respondError(w, r, http.StatusBadRequest, "parse_failed", "Invalid enrollment request")
		return
	}

	csrData, err := windows.ExtractCSR(env)
	if err != nil {
		s.logger.Error("failed to extract CSR", "error", err)
		respondError(w, r, http.StatusBadRequest, "csr_extraction_failed", "Failed to extract CSR")
		return
	}

	if s.certService == nil {
		respondError(w, r, http.StatusServiceUnavailable, "ca_unavailable", "Certificate authority not configured")
		return
	}

	// Create a pending device record
	deviceID := uuid.New()

	// PEM-encode the DER CSR for the certificate service
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrData})

	// Sign the CSR
	validity := 365 * 24 * time.Hour
	certPEM, err := s.certService.SignDeviceCSR(r.Context(), deviceID, csrPEM, validity)
	if err != nil {
		s.logger.Error("failed to sign device CSR", "error", err, "device_id", deviceID)
		respondError(w, r, http.StatusInternalServerError, "signing_failed", "Failed to sign enrollment certificate")
		return
	}

	// Parse the signed cert to get thumbprint for provisioning XML
	block, _ := decodePEMBlock(certPEM)
	if block == nil {
		s.logger.Error("failed to decode signed certificate PEM")
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to process certificate")
		return
	}

	cert, err := x509.ParseCertificate(block)
	if err != nil {
		s.logger.Error("failed to parse signed certificate", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to process certificate")
		return
	}

	// Generate provisioning XML and enrollment response
	serverURL := fmt.Sprintf("https://%s", r.Host)
	thumbprint := fmt.Sprintf("%X", sha256.Sum256(cert.Raw))
	provisioningXML := windows.GenerateProvisioningXML(serverURL, thumbprint)

	resp, err := windows.GenerateEnrollmentResponse(cert, provisioningXML)
	if err != nil {
		s.logger.Error("failed to generate enrollment response", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate enrollment response")
		return
	}

	s.logAudit(r, "enrollment.windows.complete", "device", deviceID, map[string]interface{}{
		"platform":    models.PlatformWindows,
		"cert_serial": cert.SerialNumber.String(),
	})
	if s.metrics != nil {
		s.metrics.EnrollmentsTotal.WithLabelValues(models.PlatformWindows, "complete").Inc()
	}

	s.logger.Info("windows enrollment complete",
		"device_id", deviceID,
		"cert_serial", cert.SerialNumber,
	)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Windows policy service
func (s *Server) handleWindowsPolicyService(w http.ResponseWriter, r *http.Request) {
	policy := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <GetPoliciesResponse xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy">
      <response>
        <policyID>LocalMDM</policyID>
        <policyFriendlyName>LocalMDM Enrollment Policy</policyFriendlyName>
        <nextUpdateHours>12</nextUpdateHours>
        <policiesNotChanged>false</policiesNotChanged>
        <policies>
          <policy>
            <policyOIDReference>0</policyOIDReference>
            <cAs>
              <cA>
                <uris>
                  <uri>https://` + r.Host + `/EnrollmentServer/Enrollment.svc</uri>
                </uris>
                <certificate></certificate>
              </cA>
            </cAs>
            <attributes>
              <commonName>LocalMDM</commonName>
              <policySchema>3</policySchema>
              <certificateValidity>
                <validityPeriodSeconds>31536000</validityPeriodSeconds>
                <renewalPeriodSeconds>2592000</renewalPeriodSeconds>
              </certificateValidity>
              <permission>
                <enroll>true</enroll>
                <autoEnroll>false</autoEnroll>
              </permission>
              <privateKeyAttributes>
                <minimalKeyLength>2048</minimalKeyLength>
                <keySpec>1</keySpec>
                <keyUsageProperty>160</keyUsageProperty>
                <permissions>1</permissions>
                <algorithmOIDReference>0</algorithmOIDReference>
                <cryptoProviders>
                  <provider>Microsoft Software Key Storage Provider</provider>
                </cryptoProviders>
              </privateKeyAttributes>
            </attributes>
          </policy>
        </policies>
      </response>
    </GetPoliciesResponse>
  </s:Body>
</s:Envelope>`

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(policy))
}

// Android enrollment token generation
func (s *Server) handleAndroidEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	enterpriseID, err := parseUUIDParam(r, "enterprise_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	// Verify enterprise exists
	if _, err := s.enterpriseRepo.GetByID(r.Context(), enterpriseID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, r, http.StatusNotFound, "not_found", "Enterprise not found")
			return
		}
		s.logger.Error("failed to verify enterprise", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to verify enterprise")
		return
	}

	// Generate a unique token ID for QR code reference
	tokenID := uuid.New()

	s.logAudit(r, "enrollment.android.token_created", "enterprise", enterpriseID, map[string]interface{}{
		"platform": models.PlatformAndroid,
		"token_id": tokenID.String(),
	})
	if s.metrics != nil {
		s.metrics.EnrollmentsTotal.WithLabelValues(models.PlatformAndroid, "token_created").Inc()
	}

	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"token_id":    tokenID.String(),
		"qr_code_url": fmt.Sprintf("/api/v1/android/enrollment-token/%s/qr", tokenID.String()),
		"expires_at":  time.Now().Add(30 * 24 * time.Hour),
	})
}

// Android QR code generation
func (s *Server) handleAndroidEnrollmentQR(w http.ResponseWriter, r *http.Request) {
	tokenID := mux.Vars(r)["token_id"]

	qrCode, err := android.GenerateSimpleQRCode(tokenID)
	if err != nil {
		s.logger.Error("failed to generate QR code", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate QR code")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write(qrCode)
}

// Android webhook handler with HMAC signature verification
func (s *Server) handleAndroidWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Error("failed to read webhook body", "error", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	// Verify HMAC signature if webhook secret is configured
	webhookSecret := s.config.Android.WebhookSecret
	if webhookSecret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if signature == "" {
			s.logger.Warn("android webhook missing signature")
			http.Error(w, "Missing signature", http.StatusUnauthorized)
			return
		}
		if !verifyHMAC(body, signature, []byte(webhookSecret)) {
			s.logger.Warn("android webhook invalid signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	s.logger.Info("received android webhook", "body_size", len(body))

	// Acknowledge receipt
	w.WriteHeader(http.StatusOK)
}

// verifyHMAC checks an HMAC-SHA256 signature
func verifyHMAC(payload []byte, signature string, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// decodePEMBlock extracts the DER bytes from PEM-encoded data
func decodePEMBlock(pemData []byte) ([]byte, []byte) {
	block, rest := pem.Decode(pemData)
	if block == nil {
		return nil, rest
	}
	return block.Bytes, rest
}

// --- DEP API Handlers ---

// handleDEPTokenPKI generates or retrieves the token PKI certificate for Apple portal upload.
func (s *Server) handleDEPTokenPKI(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if name == "" {
		respondError(w, r, http.StatusBadRequest, "invalid_name", "DEP name is required")
		return
	}

	if s.depService == nil {
		respondError(w, r, http.StatusServiceUnavailable, "dep_unavailable", "DEP service not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		certPEM, err := s.depService.GenerateTokenPKI(r.Context(), name)
		if err != nil {
			s.logger.Error("failed to generate DEP token PKI", "error", err, "name", name)
			respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate token PKI")
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		w.Write(certPEM)

	case http.MethodPut:
		// Accept encrypted token file from Apple portal, decrypt and store
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || len(body) == 0 {
			respondError(w, r, http.StatusBadRequest, "invalid_body", "Request body is required")
			return
		}
		// For now, store the raw token data — full PKCS7 decryption will use nanoDEP's tokenpki
		s.logger.Info("received DEP token upload", "name", name, "size", len(body))
		w.WriteHeader(http.StatusOK)
	}
}

// handleDEPAssignerProfile gets or sets the auto-assigner profile UUID.
func (s *Server) handleDEPAssignerProfile(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if s.depService == nil {
		respondError(w, r, http.StatusServiceUnavailable, "dep_unavailable", "DEP service not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		profileUUID, modTime, err := s.depService.GetAssignerProfile(r.Context(), name)
		if err != nil {
			s.logger.Error("failed to get assigner profile", "error", err)
			respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get assigner profile")
			return
		}
		respondJSON(w, r, http.StatusOK, map[string]interface{}{
			"profile_uuid": profileUUID,
			"modified_at":  modTime,
		})

	case http.MethodPut:
		var req struct {
			ProfileUUID string `json:"profile_uuid"`
		}
		if err := parseJSONBody(r, &req); err != nil || req.ProfileUUID == "" {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "profile_uuid is required")
			return
		}
		if err := s.depService.SetAssignerProfile(r.Context(), name, req.ProfileUUID); err != nil {
			s.logger.Error("failed to set assigner profile", "error", err)
			respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to set assigner profile")
			return
		}
		s.logAudit(r, "dep.assigner_profile_set", "dep_name", uuid.Nil, map[string]interface{}{
			"dep_name":     name,
			"profile_uuid": req.ProfileUUID,
		})
		respondJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleDEPDevices lists synced DEP devices.
func (s *Server) handleDEPDevices(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if s.depService == nil {
		respondError(w, r, http.StatusServiceUnavailable, "dep_unavailable", "DEP service not configured")
		return
	}

	limit, offset := parsePagination(r)
	devices, total, err := s.depService.ListDevices(r.Context(), name, limit, offset)
	if err != nil {
		s.logger.Error("failed to list DEP devices", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list DEP devices")
		return
	}

	respondPaginated(w, r, http.StatusOK, devices, total, limit, offset)
}

// Windows OMA-DM management sync endpoint
func (s *Server) handleWindowsManagementSync(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Error("failed to read management sync request", "error", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	serverURI := fmt.Sprintf("https://%s/ManagementServer/MDM.svc", r.Host)
	handler := windows.NewManagementHandler(serverURI, s.deviceRepo, s.cmdRepo, s.logger)

	resp, err := handler.HandleSyncML(r.Context(), body)
	if err != nil {
		s.logger.Error("failed to handle OMA-DM sync", "error", err)
		http.Error(w, "Failed to process sync", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.syncml.dm+xml")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}
