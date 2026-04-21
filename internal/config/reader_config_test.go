package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReaderDSN_FallbackToBaseDSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:         "writer-host",
		Port:         5432,
		User:         "admin",
		Password:     "secret",
		Database:     "mydb",
		SSLMode:      "require",
		QueryTimeout: 10 * time.Second,
	}

	assert.Equal(t, cfg.DSN(), cfg.ReaderDSN(), "ReaderDSN should equal DSN when Reader is nil")
}

func TestReaderDSN_PartialOverride(t *testing.T) {
	cfg := DatabaseConfig{
		Host:         "writer-host",
		Port:         5432,
		User:         "admin",
		Password:     "secret",
		Database:     "mydb",
		SSLMode:      "require",
		QueryTimeout: 10 * time.Second,
		Reader: &ReaderConfig{
			Host: "reader-host",
		},
	}

	dsn := cfg.ReaderDSN()
	assert.Contains(t, dsn, "host=reader-host")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=admin")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=mydb")
	assert.Contains(t, dsn, "sslmode=require")
	assert.Contains(t, dsn, "statement_timeout=10000")
}

func TestReaderDSN_FullOverride(t *testing.T) {
	cfg := DatabaseConfig{
		Host:         "writer-host",
		Port:         5432,
		User:         "admin",
		Password:     "secret",
		Database:     "mydb",
		SSLMode:      "require",
		QueryTimeout: 10 * time.Second,
		Reader: &ReaderConfig{
			Host:         "reader-host",
			Port:         5433,
			User:         "reader-user",
			Password:     "reader-pass",
			Database:     "readerdb",
			SSLMode:      "disable",
			QueryTimeout: 5 * time.Second,
		},
	}

	dsn := cfg.ReaderDSN()
	assert.Contains(t, dsn, "host=reader-host")
	assert.Contains(t, dsn, "port=5433")
	assert.Contains(t, dsn, "user=reader-user")
	assert.Contains(t, dsn, "password=reader-pass")
	assert.Contains(t, dsn, "dbname=readerdb")
	assert.Contains(t, dsn, "sslmode=disable")
	assert.Contains(t, dsn, "statement_timeout=5000")
}

func TestReaderDSN_TimeoutFallbackChain(t *testing.T) {
	t.Run("reader timeout set", func(t *testing.T) {
		cfg := DatabaseConfig{
			Host: "h", Port: 1, User: "u", Password: "p", Database: "d", SSLMode: "s",
			QueryTimeout: 10 * time.Second,
			Reader:       &ReaderConfig{QueryTimeout: 3 * time.Second},
		}
		assert.Contains(t, cfg.ReaderDSN(), "statement_timeout=3000")
	})

	t.Run("reader timeout zero falls back to parent", func(t *testing.T) {
		cfg := DatabaseConfig{
			Host: "h", Port: 1, User: "u", Password: "p", Database: "d", SSLMode: "s",
			QueryTimeout: 10 * time.Second,
			Reader:       &ReaderConfig{},
		}
		assert.Contains(t, cfg.ReaderDSN(), "statement_timeout=10000")
	})

	t.Run("both zero falls back to default 30s", func(t *testing.T) {
		cfg := DatabaseConfig{
			Host: "h", Port: 1, User: "u", Password: "p", Database: "d", SSLMode: "s",
			Reader: &ReaderConfig{},
		}
		assert.Contains(t, cfg.ReaderDSN(), "statement_timeout=30000")
	})
}

func TestReaderPoolConfig_FallbackToBase(t *testing.T) {
	cfg := DatabaseConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	maxOpen, maxIdle, maxLifetime := cfg.ReaderPoolConfig()
	assert.Equal(t, 25, maxOpen)
	assert.Equal(t, 5, maxIdle)
	assert.Equal(t, 5*time.Minute, maxLifetime)
}

func TestReaderPoolConfig_PartialOverride(t *testing.T) {
	cfg := DatabaseConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Reader: &ReaderConfig{
			MaxOpenConns: 10,
		},
	}

	maxOpen, maxIdle, maxLifetime := cfg.ReaderPoolConfig()
	assert.Equal(t, 10, maxOpen)
	assert.Equal(t, 5, maxIdle)
	assert.Equal(t, 5*time.Minute, maxLifetime)
}

func TestReaderPoolConfig_FullOverride(t *testing.T) {
	cfg := DatabaseConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Reader: &ReaderConfig{
			MaxOpenConns:    10,
			MaxIdleConns:    3,
			ConnMaxLifetime: 2 * time.Minute,
		},
	}

	maxOpen, maxIdle, maxLifetime := cfg.ReaderPoolConfig()
	assert.Equal(t, 10, maxOpen)
	assert.Equal(t, 3, maxIdle)
	assert.Equal(t, 2*time.Minute, maxLifetime)
}
