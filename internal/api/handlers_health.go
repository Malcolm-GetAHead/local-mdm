package api

import (
	"context"
	"net/http"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/auth"
)

// Health check handler
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	timeout := s.config.Server.HealthCheckTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	type healthCheck struct {
		Status    string            `json:"status"`
		Version   string            `json:"version"`
		Checks    map[string]string `json:"checks"`
		Timestamp time.Time         `json:"timestamp"`
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

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	timeout := s.config.Server.HealthCheckTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	type depCheck struct {
		Status  string `json:"status"`
		Latency string `json:"latency"`
	}
	checks := make(map[string]depCheck)
	ready := true

	start := time.Now()
	if err := s.db.Health(ctx); err != nil {
		checks["database"] = depCheck{Status: "unhealthy", Latency: time.Since(start).String()}
		ready = false
	} else {
		checks["database"] = depCheck{Status: "healthy", Latency: time.Since(start).String()}
	}

	start = time.Now()
	if err := s.authMiddleware.HealthCheck(ctx); err != nil {
		checks["keycloak"] = depCheck{Status: "degraded", Latency: time.Since(start).String()}
	} else {
		checks["keycloak"] = depCheck{Status: "healthy", Latency: time.Since(start).String()}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}

	result := map[string]interface{}{"ready": ready, "checks": checks, "timestamp": time.Now()}
	if s.eventBus != nil {
		result["eventbus_retries_exhausted"] = s.eventBus.RetriesExhausted()
	}
	respondJSON(w, r, status, result)
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
