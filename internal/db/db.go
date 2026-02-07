package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	_ "github.com/lib/pq"
)

// DB wraps the database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection
func New(cfg config.DatabaseConfig) (*DB, error) {
	// Validate connection pool limits
	if err := validateConnectionLimits(cfg); err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(10 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// validateConnectionLimits ensures database connection pool configuration is safe
func validateConnectionLimits(cfg config.DatabaseConfig) error {
	// Validate MaxOpenConns
	if cfg.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive, got: %d", cfg.MaxOpenConns)
	}
	if cfg.MaxOpenConns > 100 {
		return fmt.Errorf("max_open_conns must not exceed 100 (PostgreSQL default limit), got: %d", cfg.MaxOpenConns)
	}

	// Validate MaxIdleConns
	if cfg.MaxIdleConns <= 0 {
		return fmt.Errorf("max_idle_conns must be positive, got: %d", cfg.MaxIdleConns)
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("max_idle_conns (%d) must not exceed max_open_conns (%d)", cfg.MaxIdleConns, cfg.MaxOpenConns)
	}

	// Validate ConnMaxLifetime
	if cfg.ConnMaxLifetime <= 0 {
		return fmt.Errorf("conn_max_lifetime must be positive, got: %v", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxLifetime < time.Minute {
		return fmt.Errorf("conn_max_lifetime must be at least 1 minute, got: %v", cfg.ConnMaxLifetime)
	}

	return nil
}

// Health checks database connectivity
func (db *DB) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
