package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
)

// ConnectDB returns a *db.DB for integration tests.
// Reads DB_HOST/DB_PASSWORD env vars (Docker) with localhost fallback.
// Skips the test if the database is unavailable.
// Registers t.Cleanup to close the connection.
func ConnectDB(t testing.TB) *db.DB {
	t.Helper()
	cfg := config.DatabaseConfig{
		Host:            envOr("DB_HOST", "localhost"),
		Port:            5432,
		User:            "postgres",
		Password:        envOr("DB_PASSWORD", "postgres-dev-password-1234"),
		Database:        "localmdm",
		SSLMode:         "disable",
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 5 * time.Minute,
	}
	database, err := db.New(cfg)
	if err != nil {
		t.Skipf("skipping: database unavailable: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// ConnectRawDB returns a *sql.DB for tests that need a raw connection
// (reporting, SCEP challenges, token cache).
// Skips the test if the database is unavailable.
// Registers t.Cleanup to close the connection.
func ConnectRawDB(t testing.TB) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("host=%s port=5432 user=postgres password=%s dbname=localmdm sslmode=disable",
		envOr("DB_HOST", "localhost"), envOr("DB_PASSWORD", "postgres-dev-password-1234"))
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("skipping: database unavailable: %v", err)
	}
	if err := d.Ping(); err != nil {
		t.Skipf("skipping: database unavailable: %v", err)
	}
	d.SetMaxOpenConns(2)
	d.SetMaxIdleConns(1)
	t.Cleanup(func() { d.Close() })
	return d
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}