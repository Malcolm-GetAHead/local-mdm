package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Database     DatabaseConfig     `yaml:"database"`
	Auth         AuthConfig         `yaml:"auth"`
	Certificates CertificatesConfig `yaml:"certificates"`
	Windows      WindowsConfig      `yaml:"windows"`
	MacOS        MacOSConfig        `yaml:"macos"`
	Android      AndroidConfig      `yaml:"android"`
	Logging      LoggingConfig      `yaml:"logging"`
	Features     FeaturesConfig     `yaml:"features"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	TLS          TLSConfig     `yaml:"tls"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
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
}

// DSN returns the database connection string
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret            string        `yaml:"jwt_secret"`
	AccessTokenDuration  time.Duration `yaml:"access_token_duration"`
	RefreshTokenDuration time.Duration `yaml:"refresh_token_duration"`
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
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("JWT secret must be set and not use default value")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	return nil
}
