package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/constants"
	_ "github.com/lib/pq"
)

// DB wraps the database connection pools for read/write splitting.
// Writer is used for all writes and transactions.
// Reader is used for read-only queries in repositories.
// In development, both pools point to the same PostgreSQL instance.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
}

// New creates writer and reader database connection pools.
// If no reader config is provided, both pools use the same DSN.
func New(cfg config.DatabaseConfig) (*DB, error) {
	if err := validateConnectionLimits(cfg); err != nil {
		return nil, err
	}

	writer, err := openPool(cfg.DSN(), cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("failed to open writer pool: %w", err)
	}

	// Validate reader pool limits
	rMaxOpen, rMaxIdle, rMaxLifetime := cfg.ReaderPoolConfig()
	rCfg := config.DatabaseConfig{
		MaxOpenConns:    rMaxOpen,
		MaxIdleConns:    rMaxIdle,
		ConnMaxLifetime: rMaxLifetime,
	}
	if err := validateConnectionLimits(rCfg); err != nil {
		writer.Close()
		return nil, fmt.Errorf("reader pool: %w", err)
	}

	reader, err := openPool(cfg.ReaderDSN(), rMaxOpen, rMaxIdle, rMaxLifetime)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to open reader pool: %w", err)
	}

	return &DB{Writer: writer, Reader: reader}, nil
}

func openPool(dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	db.SetConnMaxIdleTime(10 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// validateConnectionLimits ensures database connection pool configuration is safe
func validateConnectionLimits(cfg config.DatabaseConfig) error {
	// Validate MaxOpenConns
	if cfg.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive, got: %d", cfg.MaxOpenConns)
	}
	if cfg.MaxOpenConns > constants.MaxDatabaseConnections {
		return fmt.Errorf("max_open_conns must not exceed %d (PostgreSQL default limit), got: %d", constants.MaxDatabaseConnections, cfg.MaxOpenConns)
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

// Health checks database connectivity for both pools
func (db *DB) Health(ctx context.Context) error {
	if err := db.Writer.PingContext(ctx); err != nil {
		return fmt.Errorf("writer pool: %w", err)
	}
	if err := db.Reader.PingContext(ctx); err != nil {
		return fmt.Errorf("reader pool: %w", err)
	}
	return nil
}

// Close closes both database connection pools
func (db *DB) Close() error {
	wErr := db.Writer.Close()
	rErr := db.Reader.Close()
	if wErr != nil {
		return wErr
	}
	return rErr
}
