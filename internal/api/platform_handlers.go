package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/apperrors"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
)

// macOS enrollment profile download
func (s *Server) handleMacOSEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	enrollStart := time.Now()
	enterpriseID, err := parseUUIDParam(r, "enterprise_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	// Verify enterprise exists
	if _, err := s.enterpriseRepo.GetByID(r.Context(), enterpriseID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
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

	serverURL := fmt.Sprintf("http://%s", r.Host)
	if r.TLS != nil {
		serverURL = fmt.Sprintf("https://%s", r.Host)
	}
	scepURL := serverURL + "/scep"
	topic := s.config.MacOS.PushTopic
	orgName := "Local MDM"

	// NanoMDM handles the Apple MDM protocol; devices check in there
	nanomdmURL := s.config.MacOS.NanoMDMURL
	if nanomdmURL == "" {
		nanomdmURL = serverURL // fallback if NanoMDM not configured
	}

	// Load CA cert if available
	var caCert *x509.Certificate
	if s.certService != nil {
		caCert, _ = s.certService.GetCACertificate()
	}

	profile, err := macos.GenerateEnrollmentProfile(
		enterpriseID, nanomdmURL, scepURL, topic, challenge, orgName, caCert,
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
		s.metrics.EnrollmentDuration.WithLabelValues(models.PlatformMacOS).Observe(time.Since(enrollStart).Seconds())
	}

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", "attachment; filename=enrollment.mobileconfig")
	w.WriteHeader(http.StatusOK)
	w.Write(profile)
}

// Windows discovery service — handles MS-MDE2 SOAP discovery
func (s *Server) handleWindowsDiscoveryService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Error("failed to read discovery request", "error", err)
		respondError(w, r, http.StatusBadRequest, "read_failed", "Failed to read request")
		return
	}

	if len(body) == 0 {
		// Windows first sends a GET to check the endpoint exists, then POST with SOAP
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}

	req, messageID, err := windows.ParseDiscoverRequest(body)
	if err != nil {
		s.logger.Error("failed to parse discovery request", "error", err, "body", string(body))
		respondError(w, r, http.StatusBadRequest, "parse_failed", "Invalid discovery request")
		return
	}

	s.logger.Info("windows discovery request",
		"email", req.Request.EmailAddress,
		"device_type", req.Request.DeviceType,
		"request_version", req.Request.RequestVersion,
		"message_id", messageID,
	)

	// Detect scheme from TLS or reverse proxy header
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	enterpriseID := mux.Vars(r)["enterprise_id"]
	enrollmentURL := serverURL + "/EnrollmentServer/Enrollment.svc"
	if enterpriseID != "" {
		enrollmentURL = serverURL + "/EnrollmentServer/" + enterpriseID + "/Enrollment.svc"
	}
	policyURL := serverURL + "/EnrollmentServer/Policy.svc"

	resp, err := windows.GenerateDiscoverResponse(enrollmentURL, policyURL, messageID)
	if err != nil {
		s.logger.Error("failed to generate discovery response", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate response")
		return
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(resp)))
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Windows enrollment service — signs CSR and creates device record
func (s *Server) handleWindowsEnrollmentService(w http.ResponseWriter, r *http.Request) {
	enrollStart := time.Now()
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
		s.logger.Error("failed to parse enrollment request", "error", err, "body_len", len(body))
		respondError(w, r, http.StatusBadRequest, "parse_failed", "Invalid enrollment request")
		return
	}

	// Extract the client's MessageID for RelatesTo in response
	relatesToMessageID := env.Header.MessageID

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

	// Extract device info from AdditionalContext
	additionalCtx := windows.ExtractAdditionalContext(env)
	hwDeviceID := ""
	deviceName := ""
	osVersion := ""
	if additionalCtx != nil {
		hwDeviceID = additionalCtx.GetContextValue("DeviceID")
		deviceName = additionalCtx.GetContextValue("DeviceName")
		osVersion = additionalCtx.GetContextValue("OSVersion")
	}

	deviceID := uuid.New()

	// Use hardware DeviceID from AdditionalContext if available, otherwise use our UUID
	storedDeviceID := deviceID.String()
	if hwDeviceID != "" {
		storedDeviceID = hwDeviceID
	}

	// Determine enterprise ID from URL path, email username (UUID), or fallback
	enterpriseID := uuid.Nil
	if eidStr := mux.Vars(r)["enterprise_id"]; eidStr != "" {
		if eid, err := uuid.Parse(eidStr); err == nil {
			enterpriseID = eid
		}
	}
	if enterpriseID == uuid.Nil && env.Header.Security != nil && env.Header.Security.UsernameToken != nil {
		username := env.Header.Security.UsernameToken.Username
		if atIdx := strings.Index(username, "@"); atIdx > 0 {
			if eid, err := uuid.Parse(username[:atIdx]); err == nil {
				enterpriseID = eid
			}
		}
	}
	if enterpriseID == uuid.Nil {
		enterpriseID = uuid.MustParse("00000000-0000-0000-0000-000000000001") // default
	}

	// Create device record BEFORE signing CSR (certificates table has FK to devices)
	if enterpriseID != uuid.Nil {
		// Try to find existing device first (re-enrollment)
		existing, _ := s.deviceRepo.GetByPlatformID(r.Context(), models.PlatformWindows, storedDeviceID)
		if existing != nil {
			deviceID = existing.ID
			existing.Name = deviceName
			existing.OSVersion = osVersion
			existing.Status = models.DeviceStatusEnrolled
			_ = s.deviceRepo.Update(r.Context(), existing)
		} else {
			device := &models.Device{
				BaseModel:    models.BaseModel{ID: deviceID},
				EnterpriseID: enterpriseID,
				Platform:     models.PlatformWindows,
				DeviceID:     storedDeviceID,
				Name:         deviceName,
				OSVersion:    osVersion,
				Status:       models.DeviceStatusEnrolled,
				PlatformData: models.JSONB{},
			}
			if err := s.deviceRepo.Create(r.Context(), device); err != nil {
				s.logger.Error("failed to create windows device record", "error", err, "device_id", deviceID)
				respondError(w, r, http.StatusInternalServerError, "device_creation_failed", "Failed to create device record")
				return
			}
		}
		s.logger.Info("windows device record created",
			"device_id", deviceID,
			"hw_device_id", storedDeviceID,
			"enterprise_id", enterpriseID,
			"device_name", deviceName,
		)
	}

	// PEM-encode the DER CSR for the certificate service
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrData})

	// Sign the CSR (stores cert in DB with FK to device)
	validity := 365 * 24 * time.Hour
	certPEM, err := s.certService.SignDeviceCSR(r.Context(), deviceID, csrPEM, validity)
	if err != nil {
		s.logger.Error("failed to sign device CSR", "error", err, "device_id", deviceID)
		respondError(w, r, http.StatusInternalServerError, "signing_failed", "Failed to sign enrollment certificate")
		return
	}

	// Parse the signed cert
	block, _ := decodePEMBlock(certPEM)
	if block == nil {
		s.logger.Error("failed to decode signed certificate PEM")
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to process certificate")
		return
	}

	deviceCert, err := x509.ParseCertificate(block)
	if err != nil {
		s.logger.Error("failed to parse signed certificate", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to process certificate")
		return
	}

	// Get CA certificate for provisioning XML
	caCert, err := s.certService.GetCACertificate()
	if err != nil {
		s.logger.Error("failed to get CA certificate", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get CA certificate")
		return
	}

	// Build management URL — full path to the OMA-DM endpoint
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	managementURL := fmt.Sprintf("%s://%s/ManagementServer/MDM.svc", scheme, r.Host)

	// Generate provisioning XML with CA cert, device cert, and management URL
	provisioningXML := windows.GenerateProvisioningXML(managementURL, caCert, deviceCert)

	// Generate SOAP response with provisioning XML as the BinarySecurityToken
	resp, err := windows.GenerateEnrollmentResponse(provisioningXML, relatesToMessageID)
	if err != nil {
		s.logger.Error("failed to generate enrollment response", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate enrollment response")
		return
	}

	s.logAudit(r, "enrollment.windows.complete", "device", deviceID, map[string]interface{}{
		"platform":      models.PlatformWindows,
		"cert_serial":   deviceCert.SerialNumber.String(),
		"hw_device_id":  storedDeviceID,
	})
	if s.metrics != nil {
		s.metrics.EnrollmentsTotal.WithLabelValues(models.PlatformWindows, "complete").Inc()
		s.metrics.EnrollmentDuration.WithLabelValues(models.PlatformWindows).Observe(time.Since(enrollStart).Seconds())
	}

	s.logger.Info("windows enrollment complete",
		"device_id", deviceID,
		"hw_device_id", storedDeviceID,
		"cert_serial", deviceCert.SerialNumber,
	)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(resp)))
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Windows policy service — returns enrollment policy per MS-XCEP
func (s *Server) handleWindowsPolicyService(w http.ResponseWriter, r *http.Request) {
	// Read body to extract MessageID for RelatesTo header
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	messageID := ""
	if len(body) > 0 {
		// Quick extract of MessageID from SOAP header
		var env struct {
			Header struct {
				MessageID string `xml:"http://www.w3.org/2005/08/addressing MessageID"`
			} `xml:"Header"`
		}
		if xml.Unmarshal(body, &env) == nil {
			messageID = env.Header.MessageID
		}
	}

	policy := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope
   xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
   xmlns:s="http://www.w3.org/2003/05/soap-envelope"
   xmlns:a="http://www.w3.org/2005/08/addressing">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy/IPolicy/GetPoliciesResponse</a:Action>
    <a:RelatesTo>%s</a:RelatesTo>
  </s:Header>
  <s:Body xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xmlns:xsd="http://www.w3.org/2001/XMLSchema">
    <GetPoliciesResponse xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy">
      <response>
        <policyID/>
        <policyFriendlyName xsi:nil="true"/>
        <nextUpdateHours xsi:nil="true"/>
        <policiesNotChanged xsi:nil="true"/>
        <policies>
          <policy>
            <policyOIDReference>0</policyOIDReference>
            <cAs xsi:nil="true"/>
            <attributes>
              <commonName>LocalMDMEnrollment</commonName>
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
                <keySpec xsi:nil="true"/>
                <keyUsageProperty xsi:nil="true"/>
                <permissions xsi:nil="true"/>
                <algorithmOIDReference xsi:nil="true"/>
                <cryptoProviders xsi:nil="true"/>
              </privateKeyAttributes>
              <revision>
                <majorRevision>101</majorRevision>
                <minorRevision>0</minorRevision>
              </revision>
              <supersededPolicies xsi:nil="true"/>
              <privateKeyFlags xsi:nil="true"/>
              <subjectNameFlags xsi:nil="true"/>
              <enrollmentFlags xsi:nil="true"/>
              <generalFlags xsi:nil="true"/>
              <hashAlgorithmOIDReference>0</hashAlgorithmOIDReference>
              <rARequirements xsi:nil="true"/>
              <keyArchivalAttributes xsi:nil="true"/>
              <extensions xsi:nil="true"/>
            </attributes>
          </policy>
        </policies>
      </response>
      <cAs xsi:nil="true"/>
      <oIDs>
        <oID>
          <value>1.3.14.3.2.29</value>
          <group>1</group>
          <oIDReferenceID>0</oIDReferenceID>
          <defaultName>szOID_OIWSEC_sha1RSASign</defaultName>
        </oID>
      </oIDs>
    </GetPoliciesResponse>
  </s:Body>
</s:Envelope>`, messageID)

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(policy)))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(policy))
}

// Android enrollment token generation
func (s *Server) handleAndroidEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	enrollStart := time.Now()
	enterpriseID, err := parseUUIDParam(r, "enterprise_id")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid enterprise ID format")
		return
	}

	// Verify enterprise exists
	if _, err := s.enterpriseRepo.GetByID(r.Context(), enterpriseID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
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
		s.metrics.EnrollmentDuration.WithLabelValues(models.PlatformAndroid).Observe(time.Since(enrollStart).Seconds())
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

	// Dispatch to the Android webhook handler if configured
	if s.androidWebhookHandler != nil {
		// Restore body for the handler (HMAC verification consumed it)
		r.Body = io.NopCloser(bytes.NewReader(body))
		s.androidWebhookHandler.HandleWebhook(w, r)
		return
	}

	s.logger.Warn("android webhook received but handler not configured")
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

	resp, err := s.windowsMgmtHandler.HandleSyncML(r.Context(), body)
	if err != nil {
		s.logger.Error("failed to handle OMA-DM sync", "error", err)
		http.Error(w, "Failed to process sync", http.StatusInternalServerError)
		return
	}

	// Update last_seen for the syncing device
	if deviceID := windows.ExtractDeviceIDFromSyncML(body); deviceID != "" {
		if device, err := s.deviceRepo.GetByPlatformID(r.Context(), models.PlatformWindows, deviceID); err == nil {
			now := time.Now()
			device.LastSeen = &now
			_ = s.deviceRepo.Update(r.Context(), device)
		}
	}

	w.Header().Set("Content-Type", "application/vnd.syncml.dm+xml")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// --- Windows Provisioning Package Handlers ---

func (s *Server) handleWindowsPPKGGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string                `json:"name"`
		Template     string                `json:"template"`
		ServerURL    string                `json:"server_url"`
		DiscoveryURL string                `json:"discovery_url"`
		WiFi         *windows.PPKGWiFiConfig `json:"wifi,omitempty"`
		VPN          *windows.PPKGVPNConfig  `json:"vpn,omitempty"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	serverURL := req.ServerURL
	if serverURL == "" {
		serverURL = fmt.Sprintf("https://%s:%d", s.config.Server.Host, s.config.Server.Port)
	}

	cfg := windows.PPKGConfig{
		Name:         req.Name,
		ServerURL:    serverURL,
		DiscoveryURL: req.DiscoveryURL,
	}

	// Apply template defaults
	switch req.Template {
	case "enrollment_wifi":
		if req.WiFi == nil {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "wifi config required for enrollment_wifi template")
			return
		}
		cfg.WiFi = req.WiFi
	case "enrollment_wifi_vpn":
		if req.WiFi == nil || req.VPN == nil {
			respondError(w, r, http.StatusBadRequest, "validation_failed", "wifi and vpn config required for enrollment_wifi_vpn template")
			return
		}
		cfg.WiFi = req.WiFi
		cfg.VPN = req.VPN
	default:
		// enrollment_only or custom — apply whatever was provided
		cfg.WiFi = req.WiFi
		cfg.VPN = req.VPN
	}

	data, err := windows.GeneratePPKG(cfg, s.ppkgSigner)
	if err != nil {
		s.logger.Error("failed to generate ppkg", "error", err)
		respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate provisioning package")
		return
	}

	s.logAudit(r, "ppkg.generate", "windows", uuid.Nil, map[string]interface{}{
		"name":     cfg.Name,
		"template": req.Template,
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.ppkg"`, cfg.Name))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleWindowsPPKGTemplates(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, r, http.StatusOK, windows.AvailableTemplates())
}
