package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/logging"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CertificateMonitorIntegration(t *testing.T) {
	database := testutil.ConnectDB(t)

	dbHost := "localhost"
	if h := os.Getenv("DB_HOST"); h != "" {
		dbHost = h
	}
	dbPass := "postgres"
	if p := os.Getenv("DB_PASSWORD"); p != "" {
		dbPass = p
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         dbHost,
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:            dbHost,
			Port:            5432,
			User:            "postgres",
			Password:        dbPass,
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    2,
			MaxIdleConns:    1,
			ConnMaxLifetime: 5 * time.Minute,
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
			URL:          func() string { if u := os.Getenv("KEYCLOAK_URL"); u != "" { return u }; return "http://localhost:8180" }(),
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
				CheckInterval:    100 * time.Millisecond,
				WarningThreshold: 30 * 24 * time.Hour,
			},
		},
	}

	logger := logging.New(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	server, err := New(cfg, database, logger)
	require.NoError(t, err)
	require.NotNil(t, server.certMonitor, "Certificate monitor should be created")

	go func() {
		_ = server.Start()
	}()

	time.Sleep(200 * time.Millisecond)

	assert.NotNil(t, server.certMonitor)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServer_CertificateMonitorDisabled(t *testing.T) {
	database := testutil.ConnectDB(t)

	dbHost := "localhost"
	if h := os.Getenv("DB_HOST"); h != "" {
		dbHost = h
	}
	dbPass := "postgres"
	if p := os.Getenv("DB_PASSWORD"); p != "" {
		dbPass = p
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         dbHost,
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:            dbHost,
			Port:            5432,
			User:            "postgres",
			Password:        dbPass,
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    2,
			MaxIdleConns:    1,
			ConnMaxLifetime: 5 * time.Minute,
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
			URL:          func() string { if u := os.Getenv("KEYCLOAK_URL"); u != "" { return u }; return "http://localhost:8180" }(),
			Realm:        "localmdm",
			ClientID:     "localmdm-api",
			ClientSecret: "test-secret",
		},
		Certificates: config.CertificatesConfig{
			CACertPath:         "./certs/ca.crt",
			CAKeyPath:          "./certs/ca.key",
			DeviceCertValidity: 8760 * time.Hour,
			ExpirationMonitor: config.CertExpirationMonitorConfig{
				Enabled: false,
			},
		},
	}

	logger := logging.New(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	server, err := New(cfg, database, logger)
	require.NoError(t, err)
	assert.Nil(t, server.certMonitor, "Certificate monitor should NOT be created when disabled")
}
