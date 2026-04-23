package db

import (
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConnectionLimits_ValidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.DatabaseConfig
	}{
		{
			name: "typical production config",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
		},
		{
			name: "minimal config",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    1,
				MaxIdleConns:    1,
				ConnMaxLifetime: 1 * time.Minute,
			},
		},
		{
			name: "maximum allowed config",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    100,
				ConnMaxLifetime: 1 * time.Hour,
			},
		},
		{
			name: "high traffic config",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    50,
				MaxIdleConns:    10,
				ConnMaxLifetime: 10 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConnectionLimits(tt.cfg)
			assert.NoError(t, err)
		})
	}
}

func TestValidateConnectionLimits_InvalidMaxOpenConns(t *testing.T) {
	tests := []struct {
		name          string
		maxOpenConns  int
		expectedError string
	}{
		{
			name:          "zero connections",
			maxOpenConns:  0,
			expectedError: "max_open_conns must be positive",
		},
		{
			name:          "negative connections",
			maxOpenConns:  -1,
			expectedError: "max_open_conns must be positive",
		},
		{
			name:          "exceeds PostgreSQL limit",
			maxOpenConns:  101,
			expectedError: "max_open_conns must not exceed 100",
		},
		{
			name:          "way too high",
			maxOpenConns:  1000,
			expectedError: "max_open_conns must not exceed 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DatabaseConfig{
				MaxOpenConns:    tt.maxOpenConns,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			}

			err := validateConnectionLimits(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateConnectionLimits_InvalidMaxIdleConns(t *testing.T) {
	tests := []struct {
		name          string
		maxIdleConns  int
		expectedError string
	}{
		{
			name:          "zero idle connections",
			maxIdleConns:  0,
			expectedError: "max_idle_conns must be positive",
		},
		{
			name:          "negative idle connections",
			maxIdleConns:  -1,
			expectedError: "max_idle_conns must be positive",
		},
		{
			name:          "idle exceeds open",
			maxIdleConns:  30,
			expectedError: "max_idle_conns (30) must not exceed max_open_conns (25)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    tt.maxIdleConns,
				ConnMaxLifetime: 5 * time.Minute,
			}

			err := validateConnectionLimits(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateConnectionLimits_InvalidConnMaxLifetime(t *testing.T) {
	tests := []struct {
		name            string
		connMaxLifetime time.Duration
		expectedError   string
	}{
		{
			name:            "zero lifetime",
			connMaxLifetime: 0,
			expectedError:   "conn_max_lifetime must be positive",
		},
		{
			name:            "negative lifetime",
			connMaxLifetime: -1 * time.Minute,
			expectedError:   "conn_max_lifetime must be positive",
		},
		{
			name:            "too short lifetime",
			connMaxLifetime: 30 * time.Second,
			expectedError:   "conn_max_lifetime must be at least 1 minute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: tt.connMaxLifetime,
			}

			err := validateConnectionLimits(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestValidateConnectionLimits_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.DatabaseConfig
		shouldErr bool
		errMsg    string
	}{
		{
			name: "idle equals open (valid)",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    25,
				ConnMaxLifetime: 5 * time.Minute,
			},
			shouldErr: false,
		},
		{
			name: "exactly 1 minute lifetime (valid)",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 1 * time.Minute,
			},
			shouldErr: false,
		},
		{
			name: "exactly 100 connections (valid)",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 5 * time.Minute,
			},
			shouldErr: false,
		},
		{
			name: "59 seconds lifetime (invalid)",
			cfg: config.DatabaseConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 59 * time.Second,
			},
			shouldErr: true,
			errMsg:    "conn_max_lifetime must be at least 1 minute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConnectionLimits(tt.cfg)
			if tt.shouldErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNew_RejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.DatabaseConfig
		errSubstr string
	}{
		{
			name: "zero max_open_conns",
			cfg: config.DatabaseConfig{
				Host:            testDBHost(),
				Port:            5432,
				User:            "postgres",
				Password:        testDBPassword(),
				Database:        "test",
				SSLMode:         "disable",
				MaxOpenConns:    0,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
			errSubstr: "max_open_conns must be positive",
		},
		{
			name: "excessive max_open_conns",
			cfg: config.DatabaseConfig{
				Host:            testDBHost(),
				Port:            5432,
				User:            "postgres",
				Password:        testDBPassword(),
				Database:        "test",
				SSLMode:         "disable",
				MaxOpenConns:    200,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
			errSubstr: "max_open_conns must not exceed 100",
		},
		{
			name: "idle exceeds open",
			cfg: config.DatabaseConfig{
				Host:            testDBHost(),
				Port:            5432,
				User:            "postgres",
				Password:        testDBPassword(),
				Database:        "test",
				SSLMode:         "disable",
				MaxOpenConns:    25,
				MaxIdleConns:    30,
				ConnMaxLifetime: 5 * time.Minute,
			},
			errSubstr: "max_idle_conns (30) must not exceed max_open_conns (25)",
		},
		{
			name: "zero lifetime",
			cfg: config.DatabaseConfig{
				Host:            testDBHost(),
				Port:            5432,
				User:            "postgres",
				Password:        testDBPassword(),
				Database:        "test",
				SSLMode:         "disable",
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 0,
			},
			errSubstr: "conn_max_lifetime must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := New(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
			assert.Nil(t, db)
		})
	}
}

func TestNew_AcceptsValidConfiguration(t *testing.T) {
	// Note: This test will fail if PostgreSQL is not running
	// In CI/CD, ensure PostgreSQL is available or skip this test
	t.Skip("Requires PostgreSQL - run manually or in integration tests")

	cfg := config.DatabaseConfig{
		Host:            testDBHost(),
		Port:            5432,
		User:            "postgres",
		Password:        testDBPassword(),
		Database:        "localmdm_test",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify connection pool settings
	stats := db.Writer.Stats()
	assert.Equal(t, 25, stats.MaxOpenConnections)
}

func TestConnectionPoolBehavior(t *testing.T) {
	// This test verifies the validation logic without requiring a database
	tests := []struct {
		name        string
		maxOpen     int
		maxIdle     int
		lifetime    time.Duration
		expectError bool
	}{
		{"valid small pool", 5, 2, 5 * time.Minute, false},
		{"valid medium pool", 25, 5, 5 * time.Minute, false},
		{"valid large pool", 100, 20, 10 * time.Minute, false},
		{"invalid zero open", 0, 5, 5 * time.Minute, true},
		{"invalid negative open", -10, 5, 5 * time.Minute, true},
		{"invalid too large", 150, 5, 5 * time.Minute, true},
		{"invalid zero idle", 25, 0, 5 * time.Minute, true},
		{"invalid idle > open", 25, 30, 5 * time.Minute, true},
		{"invalid zero lifetime", 25, 5, 0, true},
		{"invalid short lifetime", 25, 5, 30 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DatabaseConfig{
				MaxOpenConns:    tt.maxOpen,
				MaxIdleConns:    tt.maxIdle,
				ConnMaxLifetime: tt.lifetime,
			}

			err := validateConnectionLimits(cfg)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
