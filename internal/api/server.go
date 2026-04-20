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
	"github.com/malcolm-getahead/local-mdm/internal/metrics"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
)

// Server represents the HTTP server
type Server struct {
	router           *mux.Router
	db               *db.DB
	config           *config.Config
	logger           *slog.Logger
	authMiddleware   *auth.Middleware
	auditLogger      audit.AuditLogger
	server           *http.Server
	authRateLimiter  *authRateLimiter
	enrollmentLimiter *rateLimiter
	metrics           *metrics.Metrics
	metricsServer     *metrics.Server
	certMonitor      *certs.ExpirationMonitor
	challengeManager *scep.ChallengeManager
	deviceRepo       repository.DeviceRepository
	enterpriseRepo   repository.EnterpriseRepository
	policyRepo       repository.PolicyRepository
	transactor       repository.Transactor
	certService      *certs.CertificateService
	certRepo         repository.CertificateRepository
	auditLogRepo     repository.AuditLogRepository
	cmdRepo          repository.CommandRepository
	appRepo          repository.AppRepository
	depService       *macos.DEPService
	depSyncCancel    context.CancelFunc
	macosService     *macos.Service
	nanomdmService   *macos.NanoMDMService
	windowsService   *windows.Service
	windowsMgmtHandler *windows.ManagementHandler
	ppkgSigner         *windows.PPKGSigner
	androidService   *android.Service
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
		router:           mux.NewRouter(),
		db:               database,
		config:           cfg,
		logger:           logger,
		challengeManager: scep.NewChallengeManager(),
	}

	// Initialize audit logger (conditional on feature flag)
	if cfg.Features.EnableAuditLog {
		s.auditLogger = audit.NewAsyncLogger(database.DB, bufferSize, workerCount, logger)
		logger.Info("Audit logging enabled")
	} else {
		s.auditLogger = audit.NopAuditLogger{}
	}

	// Initialize repositories
	deviceRepo, err := repository.NewDeviceRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create device repository: %w", err)
	}
	s.deviceRepo = deviceRepo

	enterpriseRepo, err := repository.NewEnterpriseRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create enterprise repository: %w", err)
	}
	s.enterpriseRepo = enterpriseRepo

	policyRepo, err := repository.NewPolicyRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy repository: %w", err)
	}
	s.policyRepo = policyRepo

	transactor, err := repository.NewTransactor(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	s.transactor = transactor

	certRepo, err := repository.NewCertificateRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate repository: %w", err)
	}
	s.certRepo = certRepo

	auditLogRepo, err := repository.NewAuditLogRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log repository: %w", err)
	}
	s.auditLogRepo = auditLogRepo

	cmdRepo, err := repository.NewCommandRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create command repository: %w", err)
	}
	s.cmdRepo = cmdRepo

	appRepo, err := repository.NewAppRepository(database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create app repository: %w", err)
	}
	s.appRepo = appRepo

	// Initialize certificate service
	caManager, err := certs.NewCAManager(cfg.Certificates.CACertPath, cfg.Certificates.CAKeyPath)
	if err != nil {
		logger.Warn("CA manager not available, certificate operations disabled", "error", err)
	} else {
		s.certService = certs.NewCertificateService(caManager, database.DB)
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
	
	// Enrollment rate limiter: 10 requests per minute per IP
	s.enrollmentLimiter = newRateLimiter(10, time.Minute)

	// Initialize metrics
	if cfg.Metrics.Enabled {
		s.metrics = metrics.New(database.DB)
		s.metricsServer = metrics.NewServer(cfg.Metrics.Host, cfg.Metrics.Port, s.metrics, logger)
		logger.Info("Metrics enabled", "host", cfg.Metrics.Host, "port", cfg.Metrics.Port)
	}

	// Initialize DEP service if encryption key is configured
	if cfg.MacOS.DEPEncryptionKey != "" {
		depStorage := macos.NewDEPStorage(database.DB, cfg.MacOS.DEPEncryptionKey)
		s.depService = macos.NewDEPService(depStorage, logger)
		logger.Info("DEP service initialized")
	}

	// Initialize platform services
	s.macosService = macos.NewService(s.deviceRepo)

	s.nanomdmService = macos.NewNanoMDMService(
		cfg.MacOS.NanoMDMURL, cfg.MacOS.NanoMDMAPIKey,
		s.cmdRepo, s.deviceRepo, logger,
	)

	s.windowsService = windows.NewService(s.deviceRepo)

	mgmtURI := cfg.Windows.ManagementURL
	if mgmtURI == "" {
		mgmtURI = fmt.Sprintf("https://%s:%d/ManagementServer/MDM.svc", cfg.Server.Host, cfg.Server.Port)
	}
	s.windowsMgmtHandler = windows.NewManagementHandler(mgmtURI, s.deviceRepo, s.cmdRepo, logger)

	// Initialize PPKG signer (auto-generates dev cert if paths configured but files missing)
	if cfg.Windows.PPKGSigningCert != "" && cfg.Windows.PPKGSigningKey != "" {
		signer, err := windows.NewPPKGSigner(cfg.Windows.PPKGSigningCert, cfg.Windows.PPKGSigningKey, true)
		if err != nil {
			logger.Warn("PPKG signing not available", "error", err)
		} else {
			s.ppkgSigner = signer
			logger.Info("PPKG signing initialized")
		}
	}

	s.androidService = android.NewService(s.deviceRepo, s.enterpriseRepo, cfg.Android.ProjectID, cfg.Android.ServiceAccountJSON)

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
			ipAllowlistMiddleware(s.config.Admin.AllowedIPs)(
				http.HandlerFunc(s.handleCreateEnterprise),
			),
		),
	)).Methods("POST")
	
	api.Handle("/enterprises/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetEnterprise),
	)).Methods("GET")

	api.Handle("/enterprises/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleUpdateEnterprise),
		),
	)).Methods("PUT")

	api.Handle("/enterprises/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("super_admin")(
			ipAllowlistMiddleware(s.config.Admin.AllowedIPs)(
				http.HandlerFunc(s.handleDeleteEnterprise),
			),
		),
	)).Methods("DELETE")

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
			ipAllowlistMiddleware(s.config.Admin.AllowedIPs)(
				http.HandlerFunc(s.handleWipeDevice),
			),
		),
	)).Methods("POST")

	api.Handle("/devices/{id}/restart", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleRestartDevice),
		),
	)).Methods("POST")

	api.Handle("/devices/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUpdateDevice),
		),
	)).Methods("PUT")

	api.Handle("/devices/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin")(
			http.HandlerFunc(s.handleDeleteDevice),
		),
	)).Methods("DELETE")

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

	api.Handle("/policies/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUpdatePolicy),
		),
	)).Methods("PUT")

	api.Handle("/policies/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin")(
			http.HandlerFunc(s.handleDeletePolicy),
		),
	)).Methods("DELETE")

	api.Handle("/policies/{id}/assign", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleAssignPolicy),
		),
	)).Methods("POST")

	api.Handle("/policies/{id}/assign/{device_id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUnassignPolicy),
		),
	)).Methods("DELETE")

	// Certificates
	api.Handle("/certificates", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListCertificates),
	)).Methods("GET")
	
	// Device commands (Sprint 3)
	api.Handle("/devices/{id}/commands", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleSendCommand),
		),
	)).Methods("POST")

	api.Handle("/devices/{id}/commands", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListCommands),
	)).Methods("GET")

	// Device profiles (Sprint 3)
	api.Handle("/devices/{id}/profiles", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleInstallProfile),
		),
	)).Methods("POST")

	api.Handle("/devices/{id}/profiles/{profile_id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleRemoveProfile),
		),
	)).Methods("DELETE")
	
	// Audit logs
	api.Handle("/audit-logs", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleListAuditLogs),
		),
	)).Methods("GET")

	// App catalog (Sprint 3)
	api.Handle("/apps", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListApps),
	)).Methods("GET")

	api.Handle("/apps", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleCreateApp),
		),
	)).Methods("POST")

	api.Handle("/apps/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetApp),
	)).Methods("GET")

	api.Handle("/apps/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUpdateApp),
		),
	)).Methods("PUT")

	api.Handle("/apps/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin")(
			http.HandlerFunc(s.handleDeleteApp),
		),
	)).Methods("DELETE")

	api.Handle("/apps/{id}/deploy", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleDeployApp),
		),
	)).Methods("POST")

	// Platform-specific routes (Sprint 2)
	enrollLimiter := rateLimitMiddleware(s.enrollmentLimiter)
	
	// macOS MDM endpoints
	api.Handle("/macos/enroll/{enterprise_id}", enrollLimiter(http.HandlerFunc(s.handleMacOSEnrollmentProfile))).Methods("GET")

	// macOS DEP endpoints
	api.Handle("/dep/{name}/tokenpki", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleDEPTokenPKI),
		),
	)).Methods("GET", "PUT")
	api.Handle("/dep/{name}/assigner", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleDEPAssignerProfile),
		),
	)).Methods("GET", "PUT")
	api.Handle("/dep/{name}/devices", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleDEPDevices),
	)).Methods("GET")
	checkinHandler := macos.NewCheckinHandler(s.nanomdmService, s.macosService, s.logger)
	commandHandler := macos.NewCommandHandler(s.nanomdmService, s.logger)
	s.router.Handle("/mdm", commandHandler).Methods("PUT")
	s.router.Handle("/checkin", checkinHandler).Methods("PUT")
	
	// Windows MDM endpoints
	s.router.HandleFunc("/EnrollmentServer/Discovery.svc", s.handleWindowsDiscoveryService).Methods("GET", "POST")
	s.router.HandleFunc("/EnrollmentServer/Policy.svc", s.handleWindowsPolicyService).Methods("POST")
	s.router.Handle("/EnrollmentServer/Enrollment.svc", enrollLimiter(http.HandlerFunc(s.handleWindowsEnrollmentService))).Methods("POST")
	s.router.HandleFunc("/ManagementServer/MDM.svc", s.handleWindowsManagementSync).Methods("POST")

	// Windows provisioning packages (Sprint 3)
	api.Handle("/windows/ppkg", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleWindowsPPKGGenerate),
		),
	)).Methods("POST")

	api.HandleFunc("/windows/ppkg/templates", s.handleWindowsPPKGTemplates).Methods("GET")
	
	// Android MDM endpoints
	api.Handle("/android/enrollment-token/{enterprise_id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleAndroidEnrollmentToken),
	)).Methods("POST")
	api.HandleFunc("/android/enrollment-token/{token_id}/qr", s.handleAndroidEnrollmentQR).Methods("GET")
	api.Handle("/android/webhook", enrollLimiter(http.HandlerFunc(s.handleAndroidWebhook))).Methods("POST")
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Metrics - apply first to capture all requests
	if s.metrics != nil {
		s.router.Use(s.metrics.Middleware)
	}

	// Tracing - apply early to capture all requests
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
	// Start metrics server if configured
	if s.metricsServer != nil {
		s.metricsServer.Start()
	}

	// Start certificate expiration monitor if configured
	if s.certMonitor != nil {
		s.certMonitor.Start()
		s.logger.Info("Certificate expiration monitor started")
	}

	// Start DEP sync loop if configured
	if s.depService != nil {
		interval := s.config.MacOS.DEPSyncInterval
		if interval <= 0 {
			interval = 30 * time.Minute
		}
		s.depSyncCancel = s.depService.StartDEPSync("default", interval)
		s.logger.Info("DEP sync loop started", "interval", interval)
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

	// Stop DEP sync loop
	if s.depSyncCancel != nil {
		s.depSyncCancel()
		s.logger.Info("DEP sync loop stopped")
	}
	
	// Stop auth rate limiter background goroutines
	if s.authRateLimiter != nil {
		s.authRateLimiter.Stop()
	}
	
	// Stop enrollment rate limiter
	if s.enrollmentLimiter != nil {
		s.enrollmentLimiter.Stop()
	}

	// Stop metrics server
	if s.metricsServer != nil {
		if err := s.metricsServer.Shutdown(ctx); err != nil {
			s.logger.Warn("Metrics server shutdown error", "error", err)
		}
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

