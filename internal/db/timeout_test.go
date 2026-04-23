package db

import (
	"os"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_QueryTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database integration test in short mode")
	}

	t.Run("long query is killed by statement timeout", func(t *testing.T) {
		// Create a new connection with 1 second timeout
		cfg := config.DatabaseConfig{
			Host:            testDBHost(),
			Port:            5432,
			User:            "postgres",
			Password:        testDBPassword(),
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 1 * time.Hour,
			QueryTimeout:    1 * time.Second,
		}

		db, err := New(cfg)
		require.NoError(t, err)
		defer db.Close()

		// Try to run a query that takes longer than timeout
		ctx := context.Background()
		_, err = db.Writer.ExecContext(ctx, "SELECT pg_sleep(5)")

		// Should fail with timeout error
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "timeout")
	})

	t.Run("short query completes successfully", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Host:            testDBHost(),
			Port:            5432,
			User:            "postgres",
			Password:        testDBPassword(),
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 1 * time.Hour,
			QueryTimeout:    5 * time.Second,
		}

		db, err := New(cfg)
		require.NoError(t, err)
		defer db.Close()

		// Run a quick query
		ctx := context.Background()
		_, err = db.Writer.ExecContext(ctx, "SELECT 1")

		// Should succeed
		require.NoError(t, err)
	})

	t.Run("timeout applies to all connections in pool", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Host:            testDBHost(),
			Port:            5432,
			User:            "postgres",
			Password:        testDBPassword(),
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    3,
			MaxIdleConns:    2,
			ConnMaxLifetime: 1 * time.Hour,
			QueryTimeout:    1 * time.Second,
		}

		db, err := New(cfg)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		// Run slow queries on multiple connections
		for i := 0; i < 5; i++ {
			_, err = db.Writer.ExecContext(ctx, "SELECT pg_sleep(3)")
			require.Error(t, err, "attempt %d should timeout", i+1)
			assert.Contains(t, strings.ToLower(err.Error()), "timeout")
		}
	})

	t.Run("timeout prevents connection pool exhaustion", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Host:            testDBHost(),
			Port:            5432,
			User:            "postgres",
			Password:        testDBPassword(),
			Database:        "localmdm",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 1 * time.Hour,
			QueryTimeout:    2 * time.Second,
		}

		db, err := New(cfg)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		// Start multiple slow queries concurrently
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func() {
				_, err := db.Writer.ExecContext(ctx, "SELECT pg_sleep(10)")
				assert.Error(t, err) // Should timeout
				done <- true
			}()
		}

		// Wait for all queries to timeout
		for i := 0; i < 5; i++ {
			<-done
		}

		// Connection pool should still be available
		_, err = db.Writer.ExecContext(ctx, "SELECT 1")
		require.NoError(t, err, "connection pool should be available after timeouts")
	})
}

func testDBHost() string {
	if h := os.Getenv("DB_HOST"); h != "" {
		return h
	}
	return "localhost"
}

func testDBPassword() string {
	if p := os.Getenv("DB_PASSWORD"); p != "" {
		return p
	}
	return "postgres"
}
