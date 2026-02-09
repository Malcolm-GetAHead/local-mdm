package api

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
)

// Platform services (will be initialized in server setup)
type platformServices struct {
	macOS   *macos.Service
	windows *windows.Service
	android *android.Service
}

// macOS enrollment profile download
func (s *Server) handleMacOSEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	enterpriseIDStr := vars["enterprise_id"]
	
	enterpriseID, err := uuid.Parse(enterpriseIDStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_enterprise_id", "Invalid enterprise ID format")
		return
	}

	// Generate unique SCEP challenge with 5 minute expiration
	challenge, err := s.challengeManager.GenerateChallenge(enterpriseID.String(), 5*time.Minute)
	if err != nil {
		s.logger.Error("failed to generate SCEP challenge", "error", err)
		respondError(w, r, http.StatusInternalServerError, "challenge_generation_failed", "Failed to generate enrollment challenge")
		return
	}

	// Generate enrollment profile
	serverURL := fmt.Sprintf("https://%s", r.Host)
	scepURL := serverURL + "/scep"
	topic := s.config.MacOS.PushTopic
	orgName := "Local MDM" // Default organization name

	// Generate profile
	profile, err := macos.GenerateEnrollmentProfile(
		enterpriseID,
		serverURL,
		scepURL,
		topic,
		challenge,
		orgName,
		nil, // CA cert - would load from cert service
	)
	if err != nil {
		s.logger.Error("failed to generate enrollment profile", "error", err)
		respondError(w, r, http.StatusInternalServerError, "generation_failed", "Failed to generate profile")
		return
	}

	s.logger.Info("generated enrollment profile",
		"enterprise_id", enterpriseID,
		"challenge_expires", time.Now().Add(5*time.Minute),
	)

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", "attachment; filename=enrollment.mobileconfig")
	w.WriteHeader(http.StatusOK)
	w.Write(profile)
}

// Windows discovery service
func (s *Server) handleWindowsDiscoveryService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
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

	// Generate discovery response
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

// Windows enrollment service
func (s *Server) handleWindowsEnrollmentService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
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

	// Extract CSR
	csrData, err := windows.ExtractCSR(env)
	if err != nil {
		s.logger.Error("failed to extract CSR", "error", err)
		respondError(w, r, http.StatusBadRequest, "csr_extraction_failed", "Failed to extract CSR")
		return
	}

	s.logger.Info("windows enrollment request", "csr_size", len(csrData))

	// In production, sign CSR with CA and create device record
	// For now, return a placeholder response
	
	respondError(w, r, http.StatusNotImplemented, "not_implemented", "Enrollment not yet implemented")
}

// Windows policy service
func (s *Server) handleWindowsPolicyService(w http.ResponseWriter, r *http.Request) {
	// Return enrollment policy
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
	vars := mux.Vars(r)
	enterpriseIDStr := vars["enterprise_id"]
	
	enterpriseID, err := uuid.Parse(enterpriseIDStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_enterprise_id", "Invalid enterprise ID format")
		return
	}

	s.logger.Info("generating android enrollment token", "enterprise_id", enterpriseID)

	// In production, call Android Management API to create token
	// For now, return a placeholder
	
	respondJSON(w, r, http.StatusOK, map[string]interface{}{
		"token":       "placeholder-token",
		"qr_code_url": fmt.Sprintf("/api/v1/android/enrollment-token/%s/qr", enterpriseID.String()),
		"expires_at":  "2026-03-08T00:00:00Z",
	})
}

// Android QR code generation
func (s *Server) handleAndroidEnrollmentQR(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tokenID := vars["token_id"]

	// In production, look up token and generate QR code
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

// Android webhook handler
func (s *Server) handleAndroidWebhook(w http.ResponseWriter, r *http.Request) {
	// In production, verify webhook signature and process event
	s.logger.Info("received android webhook")
	
	// Parse and handle webhook
	// For now, just acknowledge
	w.WriteHeader(http.StatusOK)
}
