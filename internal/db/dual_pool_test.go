package db

import (
	"context"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_Health_BothPools(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}

	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
		Database: "localmdm", SSLMode: "disable",
		MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := New(cfg)
	require.NoError(t, err)
	defer database.Close()

	err = database.Health(context.Background())
	assert.NoError(t, err)
}

func TestDB_Health_FailsWhenWriterClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}

	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
		Database: "localmdm", SSLMode: "disable",
		MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := New(cfg)
	require.NoError(t, err)
	defer database.Reader.Close()

	database.Writer.Close()
	err = database.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writer pool")
}

func TestDB_Health_FailsWhenReaderClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}

	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
		Database: "localmdm", SSLMode: "disable",
		MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := New(cfg)
	require.NoError(t, err)
	defer database.Writer.Close()

	database.Reader.Close()
	err = database.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reader pool")
}

func TestDB_Close_ClosesBothPools(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}

	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
		Database: "localmdm", SSLMode: "disable",
		MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 5 * time.Minute,
	}

	database, err := New(cfg)
	require.NoError(t, err)

	err = database.Close()
	assert.NoError(t, err)

	// Both pools should be closed — pinging either should fail
	assert.Error(t, database.Writer.Ping())
	assert.Error(t, database.Reader.Ping())
}
