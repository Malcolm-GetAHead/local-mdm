package config

import (
	"fmt"
	"os"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/constants"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Environment  string             `yaml:"environment"`  // development, staging, production
	Server       ServerConfig       `yaml:"server"`
	Database     DatabaseConfig     `yaml:"database"`
	Redis        RedisConfig        `yaml:"redis"`
	Auth         AuthConfig         `yaml:"auth"`
	Keycloak     KeycloakConfig     `yaml:"keycloak"`
	Certificates CertificatesConfig `yaml:"certificates"`
	Windows      WindowsConfig      `yaml:"windows"`
	MacOS        MacOSConfig        `yaml:"macos"`
	Android      AndroidConfig      `yaml:"android"`
	Logging      LoggingConfig      `yaml:"logging"`
	Features     FeaturesConfig     `yaml:"features"`
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
	Host           string           `yaml:"host"`
	Port           int              `yaml:"port"`
	TLS            TLSConfig        `yaml:"tls"`
	ReadTimeout    time.Duration    `yaml:"read_timeout"`
	WriteTimeout   time.Duration    `yaml:"write_timeout"`
	IdleTimeout    time.Duration    `yaml:"idle_timeout"`
	RequestTimeout time.Duration    `yaml:"request_timeout"`
	RateLimit      RateLimitConfig  `yaml:"rate_limit"`
	CORS           CORSConfig       `yaml:"cors"`
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
}

// DSN returns the database connection string
func (c DatabaseConfig) DSN() string {
	timeout := c.QueryTimeout
	if timeout == 0 {
		timeout = constants.DefaultQueryTimeout * time.Second
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s statement_timeout=%d",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode, timeout.Milliseconds())
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Addr returns the Redis address
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
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
	URL          string `yaml:"url"`
	Realm        string `yaml:"realm"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

func (c KeycloakConfig) IssuerURL() string {
	return fmt.Sprintf("%s/realms/%s", c.URL, c.Realm)
}

// CertificatesConfig holds certificate configuration
type CertificatesConfig struct {
	CACertPath          string        `yaml:"ca_cert_path"`
	CAKeyPath           string        `yaml:"ca_key_path"`
	DeviceCertValidity  time.Duration `yaml:"device_cert_validity"`
}

// WindowsConfig holds Windows MDM configuration
type WindowsConfig struct {
	DiscoveryURL  string `yaml:"discovery_url"`
	EnrollmentURL string `yaml:"enrollment_url"`
	ManagementURL string `yaml:"management_url"`
}

// MacOSConfig holds macOS MDM configuration
type MacOSConfig struct {
	APNSCertPath  string `yaml:"apns_cert_path"`
	APNSPassword  string `yaml:"apns_password"`
	PushTopic     string `yaml:"push_topic"`
	EnrollmentURL string `yaml:"enrollment_url"`
}

// AndroidConfig holds Android MDM configuration
type AndroidConfig struct {
	ProjectID          string `yaml:"project_id"`
	ServiceAccountJSON string `yaml:"service_account_json"`
	WebhookURL         string `yaml:"webhook_url"`
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
	EnableMetrics  bool `yaml:"enable_metrics"`
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
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database port is required")
	}
	if c.Server.Port == 0 {
		return fmt.Errorf("server port is required")
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
	if c.Keycloak.ClientSecret == "localmdm-api-secret" {
		return fmt.Errorf("CRITICAL: keycloak client_secret must be changed from default value")
	}
	if len(c.Keycloak.ClientSecret) < 16 {
		return fmt.Errorf("CRITICAL: keycloak client_secret must be at least 16 characters (current: %d)", len(c.Keycloak.ClientSecret))
	}

	return nil
}
