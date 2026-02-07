package config_test

import (
	"os"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Use the example config
	cfg, err := config.Load("../../configs/config.example.yaml")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify basic structure
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
}

func TestLoadConfigNotFound(t *testing.T) {
	_, err := config.Load("nonexistent.yaml")
	assert.Error(t, err)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Database: config.DatabaseConfig{
					Host: "localhost",
					Port: 5432,
				},
			},
			wantErr: false,
		},
		{
			name: "missing database host",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Database: config.DatabaseConfig{
					Port: 5432,
				},
			},
			wantErr: true,
		},
		{
			name: "missing database port",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Database: config.DatabaseConfig{
					Host: "localhost",
				},
			},
			wantErr: true,
		},
		{
			name: "missing server port",
			cfg: &config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
				},
				Database: config.DatabaseConfig{
					Host: "localhost",
					Port: 5432,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDatabaseDSN(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=testuser")
	assert.Contains(t, dsn, "password=testpass")
	assert.Contains(t, dsn, "dbname=testdb")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestKeycloakIssuerURL(t *testing.T) {
	cfg := config.KeycloakConfig{
		URL:   "http://localhost:8180",
		Realm: "localmdm",
	}

	issuerURL := cfg.IssuerURL()
	assert.Equal(t, "http://localhost:8180/realms/localmdm", issuerURL)
}

func TestEnvironmentVariableOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("DB_HOST", "override-host")
	os.Setenv("DB_PORT", "9999")
	os.Setenv("DB_USER", "override-user")
	os.Setenv("DB_PASSWORD", "override-pass")
	os.Setenv("DB_NAME", "override-db")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
	}()

	cfg, err := config.Load("../../configs/config.example.yaml")
	require.NoError(t, err)

	// Verify overrides
	assert.Equal(t, "override-host", cfg.Database.Host)
	assert.Equal(t, 9999, cfg.Database.Port)
	assert.Equal(t, "override-user", cfg.Database.User)
	assert.Equal(t, "override-pass", cfg.Database.Password)
	assert.Equal(t, "override-db", cfg.Database.Database)
}
