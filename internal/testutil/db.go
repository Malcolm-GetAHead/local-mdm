package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
)

// SetupTestDB creates a test database connection.
// Respects DB_HOST and DB_PASSWORD env vars for Docker-based testing.
func SetupTestDB(t *testing.T) *db.DB {
	t.Helper()

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	cfg := config.DatabaseConfig{
		Host:            host,
		Port:            5432,
		User:            "postgres",
		Password:        password,
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 5 * time.Minute,
	}
	
	database, err := db.New(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	
	return database
}

// CleanupTestDB closes the database connection
func CleanupTestDB(t *testing.T, database *db.DB) {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

// WithTransaction runs a test function within a transaction that is rolled back
func WithTransaction(t *testing.T, database *db.DB, fn func(ctx context.Context)) {
	t.Helper()
	
	tx, err := database.Writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Errorf("Failed to rollback transaction: %v", err)
		}
	}()
	
	ctx := context.Background()
	fn(ctx)
}
