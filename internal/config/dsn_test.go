package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_DSN_StatementTimeout(t *testing.T) {
	tests := []struct {
		name            string
		queryTimeout    time.Duration
		expectedTimeout string
	}{
		{
			name:            "explicit 10 second timeout",
			queryTimeout:    10 * time.Second,
			expectedTimeout: "statement_timeout=10000",
		},
		{
			name:            "explicit 1 minute timeout",
			queryTimeout:    1 * time.Minute,
			expectedTimeout: "statement_timeout=60000",
		},
		{
			name:            "zero timeout defaults to 30 seconds",
			queryTimeout:    0,
			expectedTimeout: "statement_timeout=30000",
		},
		{
			name:            "1 second timeout",
			queryTimeout:    1 * time.Second,
			expectedTimeout: "statement_timeout=1000",
		},
		{
			name:            "5 millisecond timeout",
			queryTimeout:    5 * time.Millisecond,
			expectedTimeout: "statement_timeout=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DatabaseConfig{
				Host:         "localhost",
				Port:         5432,
				User:         "testuser",
				Password:     "testpass",
				Database:     "testdb",
				SSLMode:      "disable",
				QueryTimeout: tt.queryTimeout,
			}

			dsn := cfg.DSN()
			assert.Contains(t, dsn, tt.expectedTimeout)
			assert.Contains(t, dsn, "host=localhost")
			assert.Contains(t, dsn, "port=5432")
			assert.Contains(t, dsn, "user=testuser")
			assert.Contains(t, dsn, "dbname=testdb")
			assert.Contains(t, dsn, "sslmode=disable")
		})
	}
}
