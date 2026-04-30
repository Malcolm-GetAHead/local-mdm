package api

import (
	"context"
	crypto_rand "crypto/rand"
	base64Std "encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
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
	"github.com/malcolm-getahead/local-mdm/internal/reporting"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/scep"
	"github.com/malcolm-getahead/local-mdm/internal/service"
)

// reportService defines the interface for report generation (allows mocking in tests).
type reportService interface {
	DeviceInventory(ctx context.Context, enterpriseID uuid.UUID, platform string) ([]reporting.DeviceRow, error)
	ComplianceReport(ctx context.Context, enterpriseID uuid.UUID) ([]reporting.ComplianceRow, error)
	EnrollmentReport(ctx context.Context, enterpriseID uuid.UUID, days int) ([]reporting.EnrollmentRow, error)
}

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
	challengeManager scep.ChallengeStore
	deviceRepo       repository.DeviceRepository
	enterpriseRepo   repository.EnterpriseRepository
	policyRepo       repository.PolicyRepository
	transactor       repository.Transactor
	certService      *certs.CertificateService
	caManager        *certs.CAManager
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
	androidWebhookHandler *android.WebhookHandler
	cmdDispatcher    *commandDispatcher
	lifecycleService *service.LifecycleService
	policyService    *service.PolicyService
	deviceService    *service.DeviceService
	appService       *service.AppService
	userService      *service.UserService
	tokenService     *service.TokenService
	reportService    reportService
	policyVersionRepo repository.PolicyVersionRepository
	groupService     *service.GroupService
	complianceService *service.ComplianceService
	eventBus              *service.EventBus
	cleanupCancel         context.CancelFunc
	webTemplates          map[string]*template.Template
	enrollmentTokenRepo   repository.EnrollmentTokenRepository
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
		challengeManager: scep.NewChallengeManager(database.Writer),
	}

	// Initialize audit logger (conditional on feature flag)
	if cfg.Features.EnableAuditLog {
		s.auditLogger = audit.NewAsyncLogger(database.Writer, bufferSize, workerCount, logger)
		logger.Info("Audit logging enabled")
	} else {
		s.auditLogger = audit.NopAuditLogger{}
	}

	// Initialize repositories
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create device repository: %w", err)
	}
	s.deviceRepo = deviceRepo

	enterpriseRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create enterprise repository: %w", err)
	}
	s.enterpriseRepo = enterpriseRepo

	policyRepo, err := repository.NewPolicyRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy repository: %w", err)
	}
	s.policyRepo = policyRepo

	transactor, err := repository.NewTransactor(database.Writer)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	s.transactor = transactor

	certRepo, err := repository.NewCertificateRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate repository: %w", err)
	}
	s.certRepo = certRepo

	auditLogRepo, err := repository.NewAuditLogRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log repository: %w", err)
	}
	s.auditLogRepo = auditLogRepo

	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create command repository: %w", err)
	}
	s.cmdRepo = cmdRepo

	appRepo, err := repository.NewAppRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create app repository: %w", err)
	}
	s.appRepo = appRepo

	// Initialize certificate service
	var caManager *certs.CAManager
	if cfg.Certificates.CACertPEM != "" && cfg.Certificates.CAKeyPEM != "" {
		// Production: CA cert/key from env vars (Secrets Manager/SSM)
		caManager, err = certs.NewCAManagerFromPEM(
			[]byte(cfg.Certificates.CACertPEM),
			[]byte(cfg.Certificates.CAKeyPEM),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA from PEM env vars: %w", err)
		}
		logger.Info("CA loaded from environment variables")
	} else {
		// Dev: CA cert/key from filesystem paths
		caManager, err = certs.NewCAManager(cfg.Certificates.CACertPath, cfg.Certificates.CAKeyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}
	s.caManager = caManager
	s.certService = certs.NewCertificateService(caManager, certs.NewSQLCertStore(database.Writer))
	
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
		
		s.certMonitor = certs.NewExpirationMonitor(database.Writer, logger, checkInterval, warningThreshold)
		logger.Info("Certificate expiration monitor configured",
			"check_interval", checkInterval,
			"warning_threshold", warningThreshold,
		)
	}
	
	// CRITICAL: Auth initialization must succeed
	validator, err := auth.NewOIDCValidator(
		cfg.Keycloak.IssuerURL(), 
		cfg.Keycloak.ClientID, 
		database.Writer, 
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
		s.metrics = metrics.New(database.Writer)
		s.metricsServer = metrics.NewServer(cfg.Metrics.Host, cfg.Metrics.Port, s.metrics, logger)
		logger.Info("Metrics enabled", "host", cfg.Metrics.Host, "port", cfg.Metrics.Port)
	}

	// Initialize DEP service if encryption key is configured
	if cfg.MacOS.DEPEncryptionKey != "" {
		depStorage := macos.NewDEPStorage(database.Writer, cfg.MacOS.DEPEncryptionKey)
		s.depService = macos.NewDEPService(depStorage, logger)
		logger.Info("DEP service initialized")
	}

	// Initialize platform services
	s.macosService = macos.NewService(s.deviceRepo)

	if cfg.MacOS.NanoMDMURL == "" {
		logger.Warn("macos.nanomdm_url not configured — macOS command delivery will fail")
	}
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
	s.lifecycleService = service.NewLifecycleService(logger)
	s.androidWebhookHandler = android.NewWebhookHandler(s.androidService, nil, logger)
	s.androidWebhookHandler.SetLifecycle(s.lifecycleService)

	s.cmdDispatcher = newCommandDispatcher(s.cmdRepo, s.nanomdmService, logger)

	policyVersionRepo, err := repository.NewPolicyVersionRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy version repository: %w", err)
	}
	s.policyVersionRepo = policyVersionRepo
	s.policyService = service.NewPolicyService(s.policyRepo, policyVersionRepo, logger)

	groupRepo, err := repository.NewGroupRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create group repository: %w", err)
	}
	assignmentRepo, err := repository.NewPolicyAssignmentRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy assignment repository: %w", err)
	}
	s.groupService = service.NewGroupService(groupRepo, assignmentRepo, logger)

	complianceRepo, err := repository.NewComplianceRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create compliance repository: %w", err)
	}
	s.complianceService = service.NewComplianceService(complianceRepo, s.groupService, s.policyRepo, s.deviceRepo, logger)

	// Initialize EventBus (LISTEN/NOTIFY on Writer pool DSN)
	s.eventBus = service.NewEventBus(cfg.Database.DSN(), database.Writer, logger)

	// Register compliance auto-evaluation subscribers
	s.eventBus.Subscribe("device.enrolled", func(ctx context.Context, event service.MDMEvent) error {
		return s.complianceService.EvaluateDeviceByID(ctx, event.ID)
	})
	s.eventBus.Subscribe("device.info_updated", func(ctx context.Context, event service.MDMEvent) error {
		return s.complianceService.EvaluateDeviceByID(ctx, event.ID)
	})
	s.eventBus.Subscribe("policy.updated", func(ctx context.Context, event service.MDMEvent) error {
		return s.complianceService.EvaluateAllForPolicy(ctx, event.ID)
	})
	s.eventBus.Subscribe("policy.assigned", func(ctx context.Context, event service.MDMEvent) error {
		if policyID, ok := event.ExtraUUID("policy_id"); ok {
			return s.complianceService.EvaluateAllForPolicy(ctx, policyID)
		}
		return nil
	})
	s.eventBus.Subscribe("policy.unassigned", func(ctx context.Context, event service.MDMEvent) error {
		// Re-evaluate affected devices so stale compliance results are cleared
		if policyID, ok := event.ExtraUUID("policy_id"); ok {
			return s.complianceService.EvaluateAllForPolicy(ctx, policyID)
		}
		return nil
	})
	s.eventBus.Subscribe("group.member_added", func(ctx context.Context, event service.MDMEvent) error {
		if event.DeviceID != nil {
			return s.complianceService.EvaluateDeviceByID(ctx, *event.DeviceID)
		}
		return nil
	})
	s.eventBus.Subscribe("group.member_removed", func(ctx context.Context, event service.MDMEvent) error {
		if event.DeviceID != nil {
			return s.complianceService.EvaluateDeviceByID(ctx, *event.DeviceID)
		}
		return nil
	})

	// Register lifecycle hooks
	s.lifecycleService.RegisterHook(service.NewComplianceCleanupHook(complianceRepo, logger))

	s.deviceService = service.NewDeviceService(s.deviceRepo, s.cmdRepo, s.cmdDispatcher, s.lifecycleService, logger)
	s.appService = service.NewAppService(s.appRepo, s.deviceRepo, s.cmdRepo, s.cmdDispatcher, logger)

	userRepo, err := repository.NewUserRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create user repository: %w", err)
	}
	s.userService = service.NewUserService(userRepo, logger)

	tokenRepo, err := repository.NewTokenRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create token repository: %w", err)
	}
	s.tokenService = service.NewTokenService(tokenRepo, userRepo, logger)
	s.reportService = reporting.NewService(reporting.NewSQLReportStore(database.Writer))

	// Initialize enrollment token repository
	enrollmentTokenRepo, err := repository.NewEnrollmentTokenRepository(database.Writer, database.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create enrollment token repository: %w", err)
	}
	s.enrollmentTokenRepo = enrollmentTokenRepo

	// Wire API token auth into middleware
	s.authMiddleware.SetTokenValidator(&tokenAuthAdapter{tokenService: s.tokenService})

	// Load dashboard templates
	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load dashboard templates: %w", err)
	}

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
	s.router.HandleFunc("/health/ready", s.handleHealthReady).Methods("GET")
	s.router.HandleFunc("/version", s.handleVersion).Methods("GET")

	// OpenAPI spec (S5-03)
	s.router.HandleFunc("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/schemas/openapi.yaml")
	}).Methods("GET")
	s.router.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/vendor/swagger-ui/index.html")
	}).Methods("GET")

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

	// Policy versioning & templates (Sprint 4)
	api.Handle("/policies/{id}/versions", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListPolicyVersions),
	)).Methods("GET")

	api.Handle("/policies/{id}/rollback", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleRollbackPolicy),
		),
	)).Methods("POST")

	api.Handle("/policies/{id}/translate", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleTranslatePolicy),
	)).Methods("GET")

	api.Handle("/policy-templates", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListPolicyTemplates),
	)).Methods("GET")

	api.Handle("/policy-templates/{id}/clone", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleClonePolicyTemplate),
		),
	)).Methods("POST")

	// Policy assignment to targets (Sprint 4)
	api.Handle("/policies/{id}/assignments", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListPolicyAssignments),
	)).Methods("GET")

	api.Handle("/policies/{id}/assignments", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleAssignPolicyToTarget),
		),
	)).Methods("POST")

	api.Handle("/policy-assignments/{assignment_id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUnassignPolicyFromTarget),
		),
	)).Methods("DELETE")

	// Device effective policies (Sprint 4)
	api.Handle("/devices/{id}/effective-policies", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetDeviceEffectivePolicies),
	)).Methods("GET")

	// Device groups (Sprint 4)
	api.Handle("/groups", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListGroups),
	)).Methods("GET")

	api.Handle("/groups", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleCreateGroup),
		),
	)).Methods("POST")

	api.Handle("/groups/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetGroup),
	)).Methods("GET")

	api.Handle("/groups/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleUpdateGroup),
		),
	)).Methods("PUT")

	api.Handle("/groups/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin")(
			http.HandlerFunc(s.handleDeleteGroup),
		),
	)).Methods("DELETE")

	api.Handle("/groups/{id}/members", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListGroupMembers),
	)).Methods("GET")

	api.Handle("/groups/{id}/members", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleAddGroupMember),
		),
	)).Methods("POST")

	api.Handle("/groups/{id}/members/{device_id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleRemoveGroupMember),
		),
	)).Methods("DELETE")

	// Compliance (Sprint 4)
	api.Handle("/compliance", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleComplianceSummary),
	)).Methods("GET")

	api.Handle("/devices/{id}/compliance", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleDeviceCompliance),
	)).Methods("GET")

	api.Handle("/devices/{id}/compliance/evaluate", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleEvaluateDeviceCompliance),
		),
	)).Methods("POST")

	// Certificates
	api.Handle("/certificates", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListCertificates),
	)).Methods("GET")

	// Users (S5-11)
	api.Handle("/users", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(http.HandlerFunc(s.handleListUsers)),
	)).Methods("GET")
	api.Handle("/users", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(http.HandlerFunc(s.handleCreateUser)),
	)).Methods("POST")
	api.Handle("/users/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleGetUser),
	)).Methods("GET")
	api.Handle("/users/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(http.HandlerFunc(s.handleUpdateUser)),
	)).Methods("PUT")
	api.Handle("/users/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(http.HandlerFunc(s.handleDeactivateUser)),
	)).Methods("DELETE")

	// API Tokens (S5-11)
	api.Handle("/tokens", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleCreateToken),
	)).Methods("POST")
	api.Handle("/tokens", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleListTokens),
	)).Methods("GET")
	api.Handle("/tokens/{id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleRevokeToken),
	)).Methods("DELETE")

	// Reports (S5-02)
	api.Handle("/reports/devices", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleDeviceReport),
	)).Methods("GET")
	api.Handle("/reports/compliance", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleComplianceReport),
	)).Methods("GET")
	api.Handle("/reports/enrollments", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleEnrollmentReport),
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

	// Enrollment tokens
	api.Handle("/enrollment-tokens", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleCreateEnrollmentToken),
		),
	)).Methods("POST")
	api.Handle("/enrollment-tokens", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleListEnrollmentTokens),
		),
	)).Methods("GET")
	api.Handle("/enrollment-tokens/{id}", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "super_admin")(
			http.HandlerFunc(s.handleRevokeEnrollmentToken),
		),
	)).Methods("DELETE")

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
	
	// SCEP endpoint (public — devices use this during enrollment)
	if s.caManager != nil {
		scepHandler := scep.NewHandler(s.caManager, s.challengeManager, s.logger)
		s.router.Handle("/scep", scepHandler).Methods("GET", "POST")
	}

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
	checkinHandler := macos.NewCheckinHandler(s.nanomdmService, s.macosService, s.cmdRepo, s.lifecycleService, s.logger)
	if s.config.MacOS.DefaultEnterpriseID != "" {
		if eid, err := uuid.Parse(s.config.MacOS.DefaultEnterpriseID); err == nil {
			checkinHandler.SetDefaultEnterpriseID(eid)
		}
	}
	commandHandler := macos.NewCommandHandler(s.nanomdmService, s.logger)
	s.router.Handle("/mdm", commandHandler).Methods("PUT")
	s.router.Handle("/checkin", checkinHandler).Methods("PUT")

	// NanoMDM webhook — receives forwarded check-in and command events
	api.Handle("/macos/webhook", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkinHandler.ServeHTTP(w, r)
	})).Methods("POST")
	
	// Windows MDM endpoints (enterprise_id in URL for multi-tenant)
	s.router.HandleFunc("/EnrollmentServer/{enterprise_id}/Discovery.svc", s.handleWindowsDiscoveryService).Methods("GET", "POST")
	s.router.HandleFunc("/EnrollmentServer/Discovery.svc", s.handleWindowsDiscoveryService).Methods("GET", "POST")
	s.router.HandleFunc("/EnrollmentServer/Policy.svc", s.handleWindowsPolicyService).Methods("POST")
	s.router.Handle("/EnrollmentServer/{enterprise_id}/Enrollment.svc", enrollLimiter(http.HandlerFunc(s.handleWindowsEnrollmentService))).Methods("POST")
	s.router.Handle("/EnrollmentServer/Enrollment.svc", enrollLimiter(http.HandlerFunc(s.handleWindowsEnrollmentService))).Methods("POST")
	s.router.HandleFunc("/ManagementServer/MDM.svc", s.handleWindowsManagementSync).Methods("POST")

	// Windows provisioning packages (Sprint 3)
	api.Handle("/windows/ppkg", s.authMiddleware.RequireAuth(
		s.authMiddleware.RequireRole("admin", "operator")(
			http.HandlerFunc(s.handleWindowsPPKGGenerate),
		),
	)).Methods("POST")

	api.Handle("/windows/ppkg/templates", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleWindowsPPKGTemplates),
	)).Methods("GET")
	
	// Android MDM endpoints
	api.Handle("/android/enrollment-token/{enterprise_id}", s.authMiddleware.RequireAuth(
		http.HandlerFunc(s.handleAndroidEnrollmentToken),
	)).Methods("POST")
	api.HandleFunc("/android/enrollment-token/{token_id}/qr", s.handleAndroidEnrollmentQR).Methods("GET")
	api.Handle("/android/webhook", enrollLimiter(http.HandlerFunc(s.handleAndroidWebhook))).Methods("POST")

	// ── Dashboard (Sprint 5d) ────────────────────────────────────────────
	// Static files
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// CRL distribution point for Windows TLS validation
	crlPath := "certs/ca.crl" // default
	if s.config.Certificates.CACertPath != "" {
		crlPath = filepath.Join(filepath.Dir(s.config.Certificates.CACertPath), "ca.crl")
	}
	s.router.HandleFunc("/crl/ca.crl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-crl")
		http.ServeFile(w, r, crlPath)
	}).Methods("GET")

	// Root redirect
	s.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusFound)
	}).Methods("GET")

	// Auth routes (no session required)
	s.router.HandleFunc("/dashboard/login", s.handleWebLogin).Methods("GET")
	s.router.HandleFunc("/dashboard/callback", s.handleWebCallback).Methods("GET")
	s.router.HandleFunc("/dashboard/logout", s.handleWebLogout).Methods("GET")

	// Protected dashboard routes
	dash := s.router.PathPrefix("/dashboard").Subrouter()
	dash.Use(s.webAuthMiddleware)
	dash.HandleFunc("/", s.handleDashboardHome).Methods("GET")
	dash.HandleFunc("/devices", s.handleWebDeviceList).Methods("GET")
	dash.HandleFunc("/devices/{id}", s.handleWebDeviceDetail).Methods("GET")
	dash.HandleFunc("/devices/{id}/lock", s.handleWebDeviceLock).Methods("POST")
	dash.HandleFunc("/devices/{id}/wipe", s.handleWebDeviceWipe).Methods("POST")
	dash.HandleFunc("/devices/{id}/unenroll", s.handleWebDeviceUnenroll).Methods("POST")
	dash.HandleFunc("/devices/{id}/checkin", s.handleWebDeviceCheckin).Methods("POST")
	dash.HandleFunc("/devices/{id}/evaluate", s.handleWebDeviceEvaluate).Methods("POST")
	dash.HandleFunc("/devices/{id}/delete", s.handleWebDeviceDelete).Methods("POST")
	dash.HandleFunc("/policies", s.handleWebPolicyList).Methods("GET")
	dash.HandleFunc("/policies/new", s.handleWebPolicyNew).Methods("GET")
	dash.HandleFunc("/policies/settings-catalog", s.handleWebSettingsCatalog).Methods("GET")
	dash.HandleFunc("/policies", s.handleWebPolicyCreate).Methods("POST")
	dash.HandleFunc("/policies/{id}", s.handleWebPolicyEdit).Methods("GET")
	dash.HandleFunc("/policies/{id}", s.handleWebPolicyUpdate).Methods("POST")
	dash.HandleFunc("/policies/{id}/delete", s.handleWebPolicyDelete).Methods("POST")
	dash.HandleFunc("/policies/{id}/assign", s.handleWebPolicyAssignPage).Methods("GET")
	dash.HandleFunc("/policies/{id}/assign", s.handleWebPolicyAssign).Methods("POST")
	dash.HandleFunc("/policies/{id}/unassign/{assignment_id}", s.handleWebPolicyUnassign).Methods("POST")
	dash.HandleFunc("/compliance", s.handleWebCompliance).Methods("GET")
	dash.HandleFunc("/groups", s.handleWebGroups).Methods("GET")
	dash.HandleFunc("/groups", s.handleWebGroupCreate).Methods("POST")
	dash.HandleFunc("/groups/{id}", s.handleWebGroupDetail).Methods("GET")
	dash.HandleFunc("/groups/{id}/edit", s.handleWebGroupEdit).Methods("POST")
	dash.HandleFunc("/groups/{id}/delete", s.handleWebGroupDelete).Methods("POST")
	dash.HandleFunc("/groups/{id}/members/{device_id}/add", s.handleWebGroupAddMember).Methods("POST")
	dash.HandleFunc("/groups/{id}/members/{device_id}/remove", s.handleWebGroupRemoveMember).Methods("POST")
	dash.HandleFunc("/audit", s.handleWebAuditLog).Methods("GET")
	dash.HandleFunc("/enrollment-tokens", s.handleWebEnrollmentTokens).Methods("GET")
	dash.HandleFunc("/enrollment-tokens", s.handleWebEnrollmentTokenCreate).Methods("POST")
	dash.HandleFunc("/enrollment-tokens/{id}/revoke", s.handleWebEnrollmentTokenRevoke).Methods("POST")
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
	s.router.Use(idempotencyMiddleware(s.db.Writer))
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
	
	// Start command dispatcher
	s.cmdDispatcher.Start()

	// Start EventBus listener
	if s.eventBus != nil {
		if err := s.eventBus.Start(context.Background()); err != nil {
			s.logger.Error("EventBus failed to start", "error", err)
		}
	}

	// Start periodic cleanup for expired token cache and idempotency keys
	s.startCleanupTicker()

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

	// Stop command dispatcher (drain queue)
	s.cmdDispatcher.Stop()

	// Stop EventBus listener
	if s.eventBus != nil {
		s.eventBus.Shutdown()
	}

	// Stop cleanup ticker
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	
	return s.server.Shutdown(ctx)
}

// startCleanupTicker runs periodic cleanup of expired token cache and idempotency keys.
func (s *Server) startCleanupTicker() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cleanupCancel = cancel

	// Refresh metrics once at startup
	s.refreshGaugeMetrics()

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := cleanupIdempotencyKeys(s.db.Writer); err != nil {
					s.logger.Warn("idempotency key cleanup failed", "error", err)
				} else if n > 0 {
					s.logger.Info("cleaned up expired idempotency keys", "count", n)
				}
				s.challengeManager.CleanupExpired()
				s.refreshGaugeMetrics()
			}
		}
	}()
	s.logger.Info("Periodic cleanup ticker started", "interval", "1h")
}

// refreshGaugeMetrics updates devices_total and certificates_expiring_soon from the database.
func (s *Server) refreshGaugeMetrics() {
	if s.metrics == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// devices_total by platform and status
	rows, err := s.db.Reader.QueryContext(ctx,
		`SELECT platform, status, COUNT(*) FROM devices WHERE deleted_at IS NULL GROUP BY platform, status`)
	if err == nil {
		s.metrics.DevicesTotal.Reset()
		defer rows.Close()
		for rows.Next() {
			var platform, status string
			var count int
			if rows.Scan(&platform, &status, &count) == nil {
				s.metrics.DevicesTotal.WithLabelValues(platform, status).Set(float64(count))
			}
		}
	}

	// certificates_expiring_soon by days bucket
	for _, days := range []int{7, 30, 90} {
		var count int
		err := s.db.Reader.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM certificates WHERE revoked_at IS NULL AND expires_at BETWEEN NOW() AND NOW() + $1::interval`,
			fmt.Sprintf("%d days", days)).Scan(&count)
		if err == nil {
			s.metrics.CertsExpiringSoon.WithLabelValues(fmt.Sprintf("%d", days)).Set(float64(count))
		}
	}
}

// Middleware

type contextKey string

const requestIDKey contextKey = "request_id"
const cspNonceKey contextKey = "csp_nonce"

func generateCSPNonce() string {
	b := make([]byte, 16)
	crypto_rand.Read(b)
	return base64Std.RawURLEncoding.EncodeToString(b)
}

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
		if strings.HasPrefix(r.URL.Path, "/docs") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:")
		} else if strings.HasPrefix(r.URL.Path, "/dashboard") || strings.HasPrefix(r.URL.Path, "/static") {
			// Generate nonce for HTMX inline styles
			nonce := generateCSPNonce()
			r = r.WithContext(context.WithValue(r.Context(), cspNonceKey, nonce))
			w.Header().Set("Content-Security-Policy", fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'nonce-%s'; img-src 'self' data:; connect-src 'self'", nonce, nonce))
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
		}
		
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


// tokenAuthAdapter bridges service.TokenService to auth.TokenValidator.
type tokenAuthAdapter struct {
	tokenService *service.TokenService
}

func (a *tokenAuthAdapter) Validate(ctx context.Context, plaintext string) (*auth.AuthUser, error) {
	user, _, err := a.tokenService.Validate(ctx, plaintext)
	if err != nil {
		return nil, err
	}
	return &auth.AuthUser{
		ID:           user.ID.String(),
		Email:        user.Email,
		Roles:        []string{user.Role},
		EnterpriseID: user.EnterpriseID,
	}, nil
}
