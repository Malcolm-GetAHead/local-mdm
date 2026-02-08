package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/audit"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/constants"
	"github.com/malcolm-getahead/local-mdm/internal/db"
)

// Server represents the HTTP server
type Server struct {
	router          *mux.Router
	db              *db.DB
	config          *config.Config
	logger          *slog.Logger
	authMiddleware  *auth.Middleware
	auditLogger     audit.AuditLogger
	server          *http.Server
	authRateLimiter *authRateLimiter
	certMonitor     *certs.ExpirationMonitor
}

// New creates a new API server
func New(cfg *config.Config, database *db.DB, logger *slog.Logger) (*Server, error) {
	// Get audit log config with defaults
	bufferSize := cfg.Auth.AuditLog.BufferSize
	if bufferSize == 0 {
		bufferSize = 1000 // Default
	}
	workerCount := cfg.Auth.AuditLog.WorkerCount
	if workerCount == 0 {
		workerCount = 3 // Default
	}

	s := &Server{
		router:      mux.NewRouter(),
		db:          database,
		config:      cfg,
		logger:      logger,
		auditLogger: audit.NewAsyncLogger(database.DB, bufferSize, workerCount, logger),
	}
	
	// Create certificate expiration monitor if enabled
	if cfg.Certificates.ExpirationMonitor.Enabled {
		checkInterval := cfg.Certificates.ExpirationMonitor.CheckInterval
		if checkInterval == 0 {
			checkInterval = 24 * time.Hour // Default: check daily
		}
		warningThreshold := cfg.Certificates.ExpirationMonitor.WarningThreshold
		if warningThreshold == 0 {
			warningThreshold = 30 * 24 * time.Hour // Default: warn 30 days before
		}
		
		s.certMonitor = certs.NewExpirationMonitor(database.DB, logger, checkInterval, warningThreshold)
		logger.Info("Certificate expiration monitor configured",
			"check_interval", checkInterval,
			"warning_threshold", warningThreshold,
		)
	}
	
	// CRITICAL: Auth initialization must succeed
	validator, err := auth.NewOIDCValidator(
		cfg.Keycloak.IssuerURL(), 
		cfg.Keycloak.ClientID, 
		cfg.Redis.Addr(), 
		cfg.Auth.CircuitBreaker.MaxFailures,
		cfg.Auth.CircuitBreaker.Timeout,
		cfg.Auth.TokenCache.TTL,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("CRITICAL: Cannot start server without authentication: %w", err)
	}
	s.authMiddleware = auth.NewMiddleware(validator, logger)
	s.authMiddleware.SetAuditLogger(s.auditLogger)
	
	// Initialize strict rate limiter for auth endpoints
	// IP-based: 10 attempts per minute
	// Account-based: 5 attempts per 5 minutes
	s.authRateLimiter = newAuthRateLimiter(10, time.Minute, 5, 5*time.Minute)
	
	s.setupRoutes()
	s.setupMiddleware()

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return s, nil
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Public routes (no auth)
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/version", s.handleVersion).Methods("GET")

	// API v1 routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Auth routes (no auth required, but strict rate limiting)
	authLimiter := authRateLimitMiddleware(s.authRateLimiter)
	api.Handle("/auth/login", authLimiter(http.HandlerFunc(s.handleLogin))).Methods("POST")
	api.Handle("/auth/refresh", authLimiter(http.HandlerFunc(s.handleRefresh))).Methods("POST")
	
	// Protected routes (require auth)
	// Enterprises
	api.Handle("/enterprises", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("super_admin", "admin")(
			http.HandlerFunc(s.handleListEnterprises),
		),
	)).Methods("GET")
	
	api.Handle("/enterprises", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("super_admin")(
			http.HandlerFunc(s.handleCreateEnterprise),
		),
	)).Methods("POST")
	
	api.Handle("/enterprises/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetEnterprise),
	)).Methods("GET")
	
	// Devices
	api.Handle("/devices", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListDevices),
	)).Methods("GET")
	
	api.Handle("/devices/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetDevice),
	)).Methods("GET")
	
	api.Handle("/devices/{id}/lock", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleLockDevice),
		),
	)).Methods("POST")
	
	api.Handle("/devices/{id}/wipe", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin")(
			http.HandlerFunc(s.handleWipeDevice),
		),
	)).Methods("POST")
	
	// Policies
	api.Handle("/policies", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListPolicies),
	)).Methods("GET")
	
	api.Handle("/policies", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleCreatePolicy),
		),
	)).Methods("POST")
	
	api.Handle("/policies/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetPolicy),
	)).Methods("GET")
	
	// Certificates
	api.Handle("/certificates", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListCertificates),
	)).Methods("GET")
	
	// Audit logs
	api.Handle("/audit-logs", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleListAuditLogs),
		),
	)).Methods("GET")

	// Platform-specific routes (implemented in Sprint 2)
	s.router.HandleFunc("/windows/discovery", s.handleWindowsDiscovery).Methods("GET")
	s.router.HandleFunc("/macos/enroll/{token}", s.handleMacOSEnroll).Methods("GET")
	s.router.HandleFunc("/android/enroll/{token}/qr", s.handleAndroidQR).Methods("GET")
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Tracing - apply first to capture all requests
	s.router.Use(tracingMiddleware)

	// Request size limit - apply second to reject large requests early
	s.router.Use(requestSizeLimitMiddleware(constants.MaxRequestBodySize))

	// Compression - apply third for maximum benefit
	s.router.Use(compressionMiddleware)

	// Request timeout - apply early to enforce on all requests
	timeout := s.config.Server.RequestTimeout
	if timeout == 0 {
		timeout = constants.DefaultRequestTimeout * time.Second
	}
	s.router.Use(timeoutMiddleware(timeout))
	
	// Rate limiting - apply early to protect all endpoints
	if s.config.Server.RateLimit.Enabled {
		limit := s.config.Server.RateLimit.RequestsPerMin
		if limit == 0 {
			limit = constants.DefaultRateLimit
		}
		window := s.config.Server.RateLimit.Window
		if window == 0 {
			window = time.Minute // Default
		}
		
		globalLimiter := newRateLimiter(limit, window)
		s.router.Use(rateLimitMiddleware(globalLimiter))
		s.logger.Info("Rate limiting enabled", "limit", limit, "window", window)
	}
	
	s.router.Use(requestIDMiddleware)
	s.router.Use(s.loggingMiddleware)
	s.router.Use(recoveryMiddleware(s.logger))
	s.router.Use(securityHeadersMiddleware)
	s.router.Use(corsMiddleware(s.config.Server.CORS))
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start certificate expiration monitor if configured
	if s.certMonitor != nil {
		s.certMonitor.Start()
		s.logger.Info("Certificate expiration monitor started")
	}
	
	if s.config.Server.TLS.Enabled {
		return s.server.ListenAndServeTLS(
			s.config.Server.TLS.CertFile,
			s.config.Server.TLS.KeyFile,
		)
	}
	
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop certificate expiration monitor
	if s.certMonitor != nil {
		s.certMonitor.Stop()
		s.logger.Info("Certificate expiration monitor stopped")
	}
	
	// Stop auth rate limiter background goroutines
	if s.authRateLimiter != nil {
		s.authRateLimiter.Stop()
	}
	
	// Gracefully shutdown async audit logger (drain queue with timeout)
	if asyncLogger, ok := s.auditLogger.(*audit.AsyncLogger); ok {
		if err := asyncLogger.Shutdown(ctx); err != nil {
			s.logger.Warn("Audit logger shutdown timeout", "error", err)
		}
	}
	
	return s.server.Shutdown(ctx)
}

// Middleware

type contextKey string

const requestIDKey contextKey = "request_id"

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func requestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		requestID, _ := r.Context().Value(requestIDKey).(string)
		
		s.logger.Info("HTTP request",
			"method", r.Method,
			"path", r.RequestURI,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"request_id", requestID,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID, _ := r.Context().Value(requestIDKey).(string)
					logger.Error("Panic recovered",
						"error", err,
						"path", r.URL.Path,
						"request_id", requestID,
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			if origin != "" && isAllowedOrigin(origin, cfg.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
			
			// Set allowed methods
			if len(cfg.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", joinStrings(cfg.AllowedMethods))
			}
			
			// Set allowed headers
			if len(cfg.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", joinStrings(cfg.AllowedHeaders))
			}
			
			// Set max age
			if cfg.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
			}
			
			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowed []string) bool {
	for _, o := range allowed {
		if o == "*" || o == origin {
			return true
		}
		// Support wildcard subdomains: *.example.com
		if strings.HasPrefix(o, "*.") {
			domain := strings.TrimPrefix(o, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

func joinStrings(strs []string) string {
	return strings.Join(strs, ", ")
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		
		// XSS protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Content Security Policy
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		
		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// HSTS (only if TLS is enabled)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		next.ServeHTTP(w, r)
	})
}

// Response helpers

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *ErrorInfo  `json:"error,omitempty"`
	Meta  *MetaInfo   `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type MetaInfo struct {
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id,omitempty"`
	Page       int       `json:"page,omitempty"`
	PerPage    int       `json:"per_page,omitempty"`
	Total      int       `json:"total,omitempty"`
	TotalPages int       `json:"total_pages,omitempty"`
}

func respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	requestID, _ := r.Context().Value(requestIDKey).(string)
	
	response := Response{
		Data: data,
		Meta: &MetaInfo{
			Timestamp: time.Now(),
			RequestID: requestID,
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func respondError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	requestID, _ := r.Context().Value(requestIDKey).(string)
	
	response := Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		Meta: &MetaInfo{
			Timestamp: time.Now(),
			RequestID: requestID,
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func respondNotImplemented(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusNotImplemented, "not_implemented", "This endpoint is not yet implemented")
}
