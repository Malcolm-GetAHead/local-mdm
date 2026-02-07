package testutil

import (
	"context"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
)

// SetupTestDB creates a test database connection
func SetupTestDB(t *testing.T) *db.DB {
	t.Helper()
	
	cfg := config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "postgres",
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 300,
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
	
	tx, err := database.BeginTx(context.Background(), nil)
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
