package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/constants"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Environment  string             `yaml:"environment"`  // development, staging, production
	Server       ServerConfig       `yaml:"server"`
	Database     DatabaseConfig     `yaml:"database"`
	Auth         AuthConfig         `yaml:"auth"`
	Keycloak     KeycloakConfig     `yaml:"keycloak"`
	Certificates CertificatesConfig `yaml:"certificates"`
	Windows      WindowsConfig      `yaml:"windows"`
	MacOS        MacOSConfig        `yaml:"macos"`
	Android      AndroidConfig      `yaml:"android"`
	Logging      LoggingConfig      `yaml:"logging"`
	Features     FeaturesConfig     `yaml:"features"`
	Metrics      MetricsConfig      `yaml:"metrics"`
	Tracing      TracingConfig      `yaml:"tracing"`
	Admin        AdminConfig        `yaml:"admin"`
}

// AdminConfig holds admin operation security configuration
type AdminConfig struct {
	AllowedIPs []string `yaml:"allowed_ips"`
}

// TracingConfig holds distributed tracing configuration
type TracingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Service string `yaml:"service"`
	Version string `yaml:"version"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string      `yaml:"allowed_origins"`
	AllowedMethods   []string      `yaml:"allowed_methods"`
	AllowedHeaders   []string      `yaml:"allowed_headers"`
	AllowCredentials bool          `yaml:"allow_credentials"`
	MaxAge           int           `yaml:"max_age"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host               string           `yaml:"host"`
	Port               int              `yaml:"port"`
	TLS                TLSConfig        `yaml:"tls"`
	ReadTimeout        time.Duration    `yaml:"read_timeout"`
	WriteTimeout       time.Duration    `yaml:"write_timeout"`
	IdleTimeout        time.Duration    `yaml:"idle_timeout"`
	RequestTimeout     time.Duration    `yaml:"request_timeout"`
	HealthCheckTimeout time.Duration    `yaml:"health_check_timeout"`
	RateLimit          RateLimitConfig  `yaml:"rate_limit"`
	CORS               CORSConfig       `yaml:"cors"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled       bool          `yaml:"enabled"`
	RequestsPerMin int          `yaml:"requests_per_min"`
	Window        time.Duration `yaml:"window"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"sslmode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	QueryTimeout    time.Duration `yaml:"query_timeout"`
	Reader          *ReaderConfig `yaml:"reader,omitempty"`
}

// ReaderConfig holds optional overrides for the read replica pool.
// Any field left at its zero value inherits from the parent DatabaseConfig.
type ReaderConfig struct {
	Host            string        `yaml:"host,omitempty"`
	Port            int           `yaml:"port,omitempty"`
	User            string        `yaml:"user,omitempty"`
	Password        string        `yaml:"password,omitempty"`
	Database        string        `yaml:"database,omitempty"`
	SSLMode         string        `yaml:"sslmode,omitempty"`
	MaxOpenConns    int           `yaml:"max_open_conns,omitempty"`
	MaxIdleConns    int           `yaml:"max_idle_conns,omitempty"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime,omitempty"`
	QueryTimeout    time.Duration `yaml:"query_timeout,omitempty"`
}

// DSN returns the writer database connection string
func (c DatabaseConfig) DSN() string {
	timeout := c.QueryTimeout
	if timeout == 0 {
		timeout = constants.DefaultQueryTimeout * time.Second
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s statement_timeout=%d",
		escapeDSNValue(c.Host), c.Port, escapeDSNValue(c.User), escapeDSNValue(c.Password), escapeDSNValue(c.Database), c.SSLMode, timeout.Milliseconds())
}

// escapeDSNValue wraps a DSN value in single quotes if it contains spaces or special characters.
func escapeDSNValue(s string) string {
	if strings.ContainsAny(s, " ='\\") {
		return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
	}
	return s
}

// ReaderDSN returns the reader database connection string.
// Falls back to DSN() when no reader overrides are configured.
func (c DatabaseConfig) ReaderDSN() string {
	if c.Reader == nil {
		return c.DSN()
	}
	r := c.Reader
	host := r.Host
	if host == "" {
		host = c.Host
	}
	port := r.Port
	if port == 0 {
		port = c.Port
	}
	user := r.User
	if user == "" {
		user = c.User
	}
	password := r.Password
	if password == "" {
		password = c.Password
	}
	database := r.Database
	if database == "" {
		database = c.Database
	}
	sslmode := r.SSLMode
	if sslmode == "" {
		sslmode = c.SSLMode
	}
	timeout := r.QueryTimeout
	if timeout == 0 {
		timeout = c.QueryTimeout
	}
	if timeout == 0 {
		timeout = constants.DefaultQueryTimeout * time.Second
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s statement_timeout=%d",
		escapeDSNValue(host), port, escapeDSNValue(user), escapeDSNValue(password), escapeDSNValue(database), sslmode, timeout.Milliseconds())
}

// ReaderPoolConfig returns the resolved pool settings for the reader,
// falling back to the base config values for any unset fields.
func (c DatabaseConfig) ReaderPoolConfig() (maxOpen, maxIdle int, maxLifetime time.Duration) {
	if c.Reader == nil {
		return c.MaxOpenConns, c.MaxIdleConns, c.ConnMaxLifetime
	}
	maxOpen = c.Reader.MaxOpenConns
	if maxOpen == 0 {
		maxOpen = c.MaxOpenConns
	}
	maxIdle = c.Reader.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = c.MaxIdleConns
	}
	maxLifetime = c.Reader.ConnMaxLifetime
	if maxLifetime == 0 {
		maxLifetime = c.ConnMaxLifetime
	}
	return
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret            string               `yaml:"jwt_secret"`
	AccessTokenDuration  time.Duration        `yaml:"access_token_duration"`
	RefreshTokenDuration time.Duration        `yaml:"refresh_token_duration"`
	CircuitBreaker       CircuitBreakerConfig `yaml:"circuit_breaker"`
	TokenCache           TokenCacheConfig     `yaml:"token_cache"`
	AuditLog             AuditLogConfig       `yaml:"audit_log"`
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxFailures int           `yaml:"max_failures"`
	Timeout     time.Duration `yaml:"timeout"`
}

// TokenCacheConfig holds token cache configuration
type TokenCacheConfig struct {
	TTL time.Duration `yaml:"ttl"`
}

// AuditLogConfig holds audit log configuration
type AuditLogConfig struct {
	BufferSize  int `yaml:"buffer_size"`
	WorkerCount int `yaml:"worker_count"`
}

// KeycloakConfig holds Keycloak OIDC configuration
type KeycloakConfig struct {
	URL           string `yaml:"url"`
	Realm         string `yaml:"realm"`
	ClientID      string `yaml:"client_id"`
	ClientSecret  string `yaml:"client_secret"`
	SessionSecret string `yaml:"session_secret"` // HMAC key for dashboard sessions; falls back to ClientSecret
}

func (c KeycloakConfig) IssuerURL() string {
	return fmt.Sprintf("%s/realms/%s", c.URL, c.Realm)
}

// CertificatesConfig holds certificate configuration
type CertificatesConfig struct {
	CACertPath          string                      `yaml:"ca_cert_path"`
	CAKeyPath           string                      `yaml:"ca_key_path"`
	CACertPEM           string                      `yaml:"-"` // from CA_CERT_PEM env var (production)
	CAKeyPEM            string                      `yaml:"-"` // from CA_KEY_PEM env var (production)
	DeviceCertValidity  time.Duration               `yaml:"device_cert_validity"`
	SCEPChallengeTTL    time.Duration               `yaml:"scep_challenge_ttl"`
	ExpirationMonitor   CertExpirationMonitorConfig `yaml:"expiration_monitor"`
}

// CertExpirationMonitorConfig holds certificate expiration monitor configuration
type CertExpirationMonitorConfig struct {
	Enabled          bool          `yaml:"enabled"`
	CheckInterval    time.Duration `yaml:"check_interval"`
	WarningThreshold time.Duration `yaml:"warning_threshold"`
}

// WindowsConfig holds Windows MDM configuration
type WindowsConfig struct {
	DiscoveryURL       string `yaml:"discovery_url"`
	EnrollmentURL      string `yaml:"enrollment_url"`
	ManagementURL      string `yaml:"management_url"`
	WNSClientID        string `yaml:"wns_client_id"`
	WNSClientSecret    string `yaml:"wns_client_secret"`
	PPKGSigningCert    string `yaml:"ppkg_signing_cert"`
	PPKGSigningKey     string `yaml:"ppkg_signing_key"`
}

// MacOSConfig holds macOS MDM configuration
type MacOSConfig struct {
	APNSCertPath       string        `yaml:"apns_cert_path"`
	APNSPassword       string        `yaml:"apns_password"`
	PushTopic          string        `yaml:"push_topic"`
	EnrollmentURL      string        `yaml:"enrollment_url"`
	NanoMDMURL         string        `yaml:"nanomdm_url"`
	NanoMDMAPIKey      string        `yaml:"nanomdm_api_key"`
	DEPEncryptionKey   string        `yaml:"dep_encryption_key"`
	DEPSyncInterval    time.Duration `yaml:"dep_sync_interval"`
}

// AndroidConfig holds Android MDM configuration
type AndroidConfig struct {
	ProjectID          string `yaml:"project_id"`
	ServiceAccountJSON string `yaml:"service_account_json"`
	WebhookURL         string `yaml:"webhook_url"`
	WebhookSecret      string `yaml:"webhook_secret"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

// FeaturesConfig holds feature flags
type FeaturesConfig struct {
	EnableAuditLog bool `yaml:"enable_audit_log"`
	EnableWebhooks bool `yaml:"enable_webhooks"`
}

// MetricsConfig holds metrics server configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if present
	cfg.overrideFromEnv()

	return &cfg, nil
}

// overrideFromEnv overrides config values with environment variables
func (c *Config) overrideFromEnv() {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		c.Environment = env
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		c.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		fmt.Sscanf(dbPort, "%d", &c.Database.Port)
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		c.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		c.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		c.Database.Database = dbName
	}
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		c.Auth.JWTSecret = jwtSecret
	}
	if keycloakSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET"); keycloakSecret != "" {
		c.Keycloak.ClientSecret = keycloakSecret
	}
	if depKey := os.Getenv("DEP_ENCRYPTION_KEY"); depKey != "" {
		c.MacOS.DEPEncryptionKey = depKey
	}
	// Reader pool overrides (creates reader subsection if any DB_READER_ env var is set)
	if rHost := os.Getenv("DB_READER_HOST"); rHost != "" {
		if c.Database.Reader == nil {
			c.Database.Reader = &ReaderConfig{}
		}
		c.Database.Reader.Host = rHost
	}
	if rPort := os.Getenv("DB_READER_PORT"); rPort != "" {
		if c.Database.Reader == nil {
			c.Database.Reader = &ReaderConfig{}
		}
		fmt.Sscanf(rPort, "%d", &c.Database.Reader.Port)
	}
	// CA cert/key as PEM from Secrets Manager/SSM (production)
	if certPEM := os.Getenv("CA_CERT_PEM"); certPEM != "" {
		c.Certificates.CACertPEM = certPEM
	}
	if keyPEM := os.Getenv("CA_KEY_PEM"); keyPEM != "" {
		c.Certificates.CAKeyPEM = keyPEM
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Database
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database port is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// Server
	if c.Server.Port == 0 {
		return fmt.Errorf("server port is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port)
	}

	// Keycloak
	if c.Keycloak.URL == "" {
		return fmt.Errorf("keycloak URL is required")
	}
	if c.Keycloak.Realm == "" {
		return fmt.Errorf("keycloak realm is required")
	}
	if c.Keycloak.ClientID == "" {
		return fmt.Errorf("keycloak client_id is required")
	}

	// Database pool sanity
	if c.Database.MaxOpenConns > 0 && c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("database max_idle_conns (%d) must not exceed max_open_conns (%d)",
			c.Database.MaxIdleConns, c.Database.MaxOpenConns)
	}

	// Logging level
	if c.Logging.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[c.Logging.Level] {
			return fmt.Errorf("invalid logging level: %s (must be: debug, info, warn, or error)", c.Logging.Level)
		}
	}

	// Validate environment
	if err := c.validateEnvironment(); err != nil {
		return err
	}

	// CRITICAL: Validate secrets are not using default/weak values
	if err := c.validateSecrets(); err != nil {
		return err
	}

	// CRITICAL: Validate TLS configuration
	if err := c.validateTLS(); err != nil {
		return err
	}

	return nil
}

// validateEnvironment ensures environment is set to a valid value
func (c *Config) validateEnvironment() error {
	// Default to development if not set
	if c.Environment == "" {
		c.Environment = "development"
	}

	// Validate environment value
	validEnvs := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
	}

	if !validEnvs[c.Environment] {
		return fmt.Errorf("invalid environment: %s (must be: development, staging, or production)", c.Environment)
	}

	return nil
}

// validateTLS ensures TLS is properly configured for the environment
func (c *Config) validateTLS() error {
	// CRITICAL: Production and staging MUST use TLS
	if c.Environment == "production" || c.Environment == "staging" {
		if !c.Server.TLS.Enabled {
			return fmt.Errorf("CRITICAL: TLS must be enabled in %s environment (set server.tls.enabled=true)", c.Environment)
		}

		// Validate TLS certificate files are specified
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("CRITICAL: TLS cert_file is required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("CRITICAL: TLS key_file is required when TLS is enabled")
		}
	}

	// If TLS is enabled, validate certificate files are specified
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert_file is required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key_file is required when TLS is enabled")
		}
	}

	return nil
}

// validateSecrets ensures secrets are properly configured and not using defaults
func (c *Config) validateSecrets() error {
	// Validate JWT secret
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("CRITICAL: jwt_secret is required")
	}
	if c.Auth.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("CRITICAL: jwt_secret must be changed from default value")
	}
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("CRITICAL: jwt_secret must be at least 32 characters (current: %d)", len(c.Auth.JWTSecret))
	}

	// Validate database password
	if c.Database.Password == "" {
		return fmt.Errorf("CRITICAL: database password is required")
	}
	if c.Database.Password == "postgres" {
		return fmt.Errorf("CRITICAL: database password must be changed from default value 'postgres'")
	}
	if len(c.Database.Password) < 16 {
		return fmt.Errorf("CRITICAL: database password must be at least 16 characters (current: %d)", len(c.Database.Password))
	}

	// Validate Keycloak client secret
	if c.Keycloak.ClientSecret == "" {
		return fmt.Errorf("CRITICAL: keycloak client_secret is required")
	}
	if c.Keycloak.ClientSecret == "localmdm-api-secret" || c.Keycloak.ClientSecret == "REPLACE_WITH_ENV_VAR" {
		return fmt.Errorf("CRITICAL: keycloak client_secret must be changed from default value")
	}
	if len(c.Keycloak.ClientSecret) < 16 {
		return fmt.Errorf("CRITICAL: keycloak client_secret must be at least 16 characters (current: %d)", len(c.Keycloak.ClientSecret))
	}

	return nil
}
