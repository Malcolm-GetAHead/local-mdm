package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
)

// Health check handler
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type healthCheck struct {
		Status   string            `json:"status"`
		Version  string            `json:"version"`
		Checks   map[string]string `json:"checks"`
		Timestamp time.Time        `json:"timestamp"`
	}

	checks := make(map[string]string)
	allHealthy := true

	// Check database
	if err := s.db.Health(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}

	// Check Keycloak
	if err := s.authMiddleware.HealthCheck(ctx); err != nil {
		checks["keycloak"] = "degraded: " + err.Error()
		// Don't mark as unhealthy - Keycloak issues shouldn't fail health check
	} else {
		checks["keycloak"] = "healthy"
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	health := healthCheck{
		Status:    status,
		Version:   "1.0.0",
		Checks:    checks,
		Timestamp: time.Now(),
	}

	respondJSON(w, r, httpStatus, health)
}

// Version handler
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, r, http.StatusOK, map[string]string{
		"version": "1.0.0",
		"build":   "dev",
	})
}

// Auth handlers
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	
	// Validate input
	if err := req.Validate(); err != nil {
		respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	
	kc := auth.NewKeycloakClient(
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		s.config.Keycloak.ClientSecret,
	)
	
	tokenResp, err := kc.Login(req.Username, req.Password)
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "login_failed", "Invalid credentials")
		return
	}
	
	respondJSON(w, r, http.StatusOK, tokenResp)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := parseJSONBody(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	
	// Validate input
	if req.RefreshToken == "" {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "refresh_token is required")
		return
	}
	if len(req.RefreshToken) > 2048 {
		respondError(w, r, http.StatusBadRequest, "validation_failed", "refresh_token too long")
		return
	}
	
	kc := auth.NewKeycloakClient(
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		s.config.Keycloak.ClientSecret,
	)
	
	tokenResp, err := kc.RefreshToken(req.RefreshToken)
	if err != nil {
		respondError(w, r, http.StatusUnauthorized, "refresh_failed", "Invalid refresh token")
		return
	}
	
	respondJSON(w, r, http.StatusOK, tokenResp)
}

// Enterprise handlers
func (s *Server) handleListEnterprises(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleCreateEnterprise(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleGetEnterprise(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Device handlers
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleLockDevice(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleWipeDevice(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Policy handlers
func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Certificate handlers
func (s *Server) handleListCertificates(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Audit log handlers
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Platform-specific handlers (Sprint 2)
func (s *Server) handleWindowsDiscovery(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleMacOSEnroll(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

func (s *Server) handleAndroidQR(w http.ResponseWriter, r *http.Request) {
	respondNotImplemented(w, r)
}

// Helper functions
func parseJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
