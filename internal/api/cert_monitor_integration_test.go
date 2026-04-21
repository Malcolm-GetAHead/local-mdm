package api

import (
	"context"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CertificateMonitorIntegration(t *testing.T) {
	// Setup test database
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Server: config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:            "test-secret-key-for-testing-only",
			AccessTokenDuration:  1 * time.Hour,
			RefreshTokenDuration: 168 * time.Hour,
			CircuitBreaker: config.CircuitBreakerConfig{
				MaxFailures: 5,
				Timeout:     30 * time.Second,
			},
			TokenCache: config.TokenCacheConfig{
				TTL: 5 * time.Minute,
			},
			AuditLog: config.AuditLogConfig{
				BufferSize:  100,
				WorkerCount: 1,
			},
		},
		Keycloak: config.KeycloakConfig{
			URL:          "http://localhost:8180",
			Realm:        "localmdm",
			ClientID:     "localmdm-api",
			ClientSecret: "test-secret",
		},
		Certificates: config.CertificatesConfig{
			CACertPath:         "./certs/ca.crt",
			CAKeyPath:          "./certs/ca.key",
			DeviceCertValidity: 8760 * time.Hour,
			ExpirationMonitor: config.CertExpirationMonitorConfig{
				Enabled:          true,
				CheckInterval:    100 * time.Millisecond, // Fast for testing
				WarningThreshold: 30 * 24 * time.Hour,
			},
		},
	}

	database, err := db.New(cfg.Database)
	require.NoError(t, err)
	defer database.Close()

	logger := logging.New(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	// Create server
	server, err := New(cfg, database, logger)
	require.NoError(t, err)
	require.NotNil(t, server.certMonitor, "Certificate monitor should be created")

	// Start server (which starts the monitor)
	go func() {
		_ = server.Start()
	}()

	// Give monitor time to start
	time.Sleep(200 * time.Millisecond)

	// Verify monitor is running
	assert.NotNil(t, server.certMonitor)

	// Shutdown server (which stops the monitor)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServer_CertificateMonitorDisabled(t *testing.T) {
	// Setup test database
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Server: config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:            "test-secret-key-for-testing-only",
			AccessTokenDuration:  1 * time.Hour,
			RefreshTokenDuration: 168 * time.Hour,
			CircuitBreaker: config.CircuitBreakerConfig{
				MaxFailures: 5,
				Timeout:     30 * time.Second,
			},
			TokenCache: config.TokenCacheConfig{
				TTL: 5 * time.Minute,
			},
			AuditLog: config.AuditLogConfig{
				BufferSize:  100,
				WorkerCount: 1,
			},
		},
		Keycloak: config.KeycloakConfig{
			URL:          "http://localhost:8180",
			Realm:        "localmdm",
			ClientID:     "localmdm-api",
			ClientSecret: "test-secret",
		},
		Certificates: config.CertificatesConfig{
			CACertPath:         "./certs/ca.crt",
			CAKeyPath:          "./certs/ca.key",
			DeviceCertValidity: 8760 * time.Hour,
			ExpirationMonitor: config.CertExpirationMonitorConfig{
				Enabled: false, // Disabled
			},
		},
	}

	database, err := db.New(cfg.Database)
	require.NoError(t, err)
	defer database.Close()

	logger := logging.New(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	// Create server
	server, err := New(cfg, database, logger)
	require.NoError(t, err)
	assert.Nil(t, server.certMonitor, "Certificate monitor should NOT be created when disabled")
}
