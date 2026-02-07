package config_test

import (
	"os"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Set required environment variables for testing
	os.Setenv("DB_PASSWORD", "test-password-at-least-16-chars")
	os.Setenv("JWT_SECRET", "test-jwt-secret-at-least-32-characters-long")
	os.Setenv("KEYCLOAK_CLIENT_SECRET", "test-keycloak-secret")
	defer func() {
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("KEYCLOAK_CLIENT_SECRET")
	}()

	// Use the example config
	cfg, err := config.Load("../../configs/config.example.yaml")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify basic structure
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)

	// Verify secrets were loaded from environment
	assert.Equal(t, "test-password-at-least-16-chars", cfg.Database.Password)
	assert.Equal(t, "test-jwt-secret-at-least-32-characters-long", cfg.Auth.JWTSecret)
	assert.Equal(t, "test-keycloak-secret", cfg.Keycloak.ClientSecret)
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
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
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
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
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
					Host:     "localhost",
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
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
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
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
	os.Setenv("DB_PASSWORD", "override-pass-at-least-16-chars")
	os.Setenv("DB_NAME", "override-db")
	os.Setenv("JWT_SECRET", "override-jwt-secret-at-least-32-characters-long")
	os.Setenv("KEYCLOAK_CLIENT_SECRET", "override-keycloak-secret")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("KEYCLOAK_CLIENT_SECRET")
	}()

	cfg, err := config.Load("../../configs/config.example.yaml")
	require.NoError(t, err)

	// Verify overrides
	assert.Equal(t, "override-host", cfg.Database.Host)
	assert.Equal(t, 9999, cfg.Database.Port)
	assert.Equal(t, "override-user", cfg.Database.User)
	assert.Equal(t, "override-pass-at-least-16-chars", cfg.Database.Password)
	assert.Equal(t, "override-db", cfg.Database.Database)
	assert.Equal(t, "override-jwt-secret-at-least-32-characters-long", cfg.Auth.JWTSecret)
	assert.Equal(t, "override-keycloak-secret", cfg.Keycloak.ClientSecret)
}

func TestSecretValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid secrets",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "default jwt secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "change-me-in-production",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "jwt_secret must be changed from default value",
		},
		{
			name: "empty jwt secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "jwt_secret is required",
		},
		{
			name: "short jwt secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "short",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "jwt_secret must be at least 32 characters",
		},
		{
			name: "default database password",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "postgres",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "database password must be changed from default value",
		},
		{
			name: "empty database password",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "database password is required",
		},
		{
			name: "short database password",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "short",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "strong-keycloak-secret",
				},
			},
			wantErr: true,
			errMsg:  "database password must be at least 16 characters",
		},
		{
			name: "default keycloak secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "localmdm-api-secret",
				},
			},
			wantErr: true,
			errMsg:  "keycloak client_secret must be changed from default value",
		},
		{
			name: "empty keycloak secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "",
				},
			},
			wantErr: true,
			errMsg:  "keycloak client_secret is required",
		},
		{
			name: "short keycloak secret",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Password: "strong-password-at-least-16-chars",
				},
				Auth: config.AuthConfig{
					JWTSecret: "strong-jwt-secret-at-least-32-characters-long",
				},
				Keycloak: config.KeycloakConfig{
					ClientSecret: "short",
				},
			},
			wantErr: true,
			errMsg:  "keycloak client_secret must be at least 16 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeycloakSecretEnvironmentOverride(t *testing.T) {
	// Set environment variable
	os.Setenv("KEYCLOAK_CLIENT_SECRET", "env-override-secret-value")
	defer os.Unsetenv("KEYCLOAK_CLIENT_SECRET")

	cfg, err := config.Load("../../configs/config.example.yaml")
	require.NoError(t, err)

	// Verify override
	assert.Equal(t, "env-override-secret-value", cfg.Keycloak.ClientSecret)
}
