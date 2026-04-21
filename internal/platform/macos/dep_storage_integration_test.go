package macos

import (
	"context"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/micromdm/nanodep/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.DB {
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
		ConnMaxLifetime: 5 * time.Minute,
	}
	database, err := db.New(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

const testEncKey = "test-encryption-key-for-dep"

func TestDEPStorage_AuthTokens(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "auth-test-" + time.Now().Format("150405")

	// Cleanup
	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_devices WHERE dep_name = $1", name)
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	tokens := &client.OAuth1Tokens{
		ConsumerKey:       "ck-test",
		ConsumerSecret:    "cs-test",
		AccessToken:       "at-test",
		AccessSecret:      "as-test",
		AccessTokenExpiry: time.Now().Add(24 * time.Hour).Truncate(time.Microsecond),
	}

	t.Run("store and retrieve", func(t *testing.T) {
		err := storage.StoreAuthTokens(ctx, name, tokens)
		require.NoError(t, err)

		fetched, err := storage.RetrieveAuthTokens(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, tokens.ConsumerKey, fetched.ConsumerKey)
		assert.Equal(t, tokens.ConsumerSecret, fetched.ConsumerSecret)
		assert.Equal(t, tokens.AccessToken, fetched.AccessToken)
		assert.Equal(t, tokens.AccessSecret, fetched.AccessSecret)
		assert.WithinDuration(t, tokens.AccessTokenExpiry, fetched.AccessTokenExpiry, time.Second)
	})

	t.Run("upsert overwrites", func(t *testing.T) {
		updated := &client.OAuth1Tokens{
			ConsumerKey:    "ck-updated",
			ConsumerSecret: "cs-updated",
			AccessToken:    "at-updated",
			AccessSecret:   "as-updated",
		}
		require.NoError(t, storage.StoreAuthTokens(ctx, name, updated))

		fetched, err := storage.RetrieveAuthTokens(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "ck-updated", fetched.ConsumerKey)
	})

	t.Run("retrieve nonexistent returns error", func(t *testing.T) {
		_, err := storage.RetrieveAuthTokens(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestDEPStorage_Config(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "config-test-" + time.Now().Format("150405")

	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	t.Run("store and retrieve", func(t *testing.T) {
		cfg := &client.Config{BaseURL: "https://mdmenrollment.apple.com"}
		require.NoError(t, storage.StoreConfig(ctx, name, cfg))

		fetched, err := storage.RetrieveConfig(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "https://mdmenrollment.apple.com", fetched.BaseURL)
	})

	t.Run("retrieve nonexistent returns error", func(t *testing.T) {
		_, err := storage.RetrieveConfig(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestDEPStorage_Cursor(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "cursor-test-" + time.Now().Format("150405")

	// Create the dep_name row first
	_, err := database.Writer.ExecContext(ctx, "INSERT INTO dep_names (name) VALUES ($1) ON CONFLICT DO NOTHING", name)
	require.NoError(t, err)
	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	t.Run("store and retrieve", func(t *testing.T) {
		require.NoError(t, storage.StoreCursor(ctx, name, "cursor-abc-123"))

		cursor, err := storage.RetrieveCursor(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "cursor-abc-123", cursor)
	})

	t.Run("retrieve nonexistent returns empty", func(t *testing.T) {
		cursor, err := storage.RetrieveCursor(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, "", cursor)
	})
}

func TestDEPStorage_AssignerProfile(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "assigner-test-" + time.Now().Format("150405")

	_, err := database.Writer.ExecContext(ctx, "INSERT INTO dep_names (name) VALUES ($1) ON CONFLICT DO NOTHING", name)
	require.NoError(t, err)
	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	t.Run("store and retrieve", func(t *testing.T) {
		require.NoError(t, storage.StoreAssignerProfile(ctx, name, "profile-uuid-123"))

		uuid, modTime, err := storage.RetrieveAssignerProfile(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "profile-uuid-123", uuid)
		assert.False(t, modTime.IsZero())
	})

	t.Run("retrieve nonexistent returns empty", func(t *testing.T) {
		uuid, _, err := storage.RetrieveAssignerProfile(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, "", uuid)
	})
}

func TestDEPStorage_TokenPKI(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "pki-test-" + time.Now().Format("150405")

	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	t.Run("generate stores staging cert", func(t *testing.T) {
		certPEM, err := storage.GenerateTokenPKI(ctx, name, "test-cn", 365)
		require.NoError(t, err)
		assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
	})

	t.Run("retrieve staging", func(t *testing.T) {
		certPEM, keyPEM, err := storage.RetrieveStagingTokenPKI(ctx, name)
		require.NoError(t, err)
		assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
		assert.Contains(t, string(keyPEM), "BEGIN RSA PRIVATE KEY")
	})

	t.Run("upstage promotes staging to current", func(t *testing.T) {
		err := storage.UpstageTokenPKI(ctx, name)
		require.NoError(t, err)

		certPEM, keyPEM, err := storage.RetrieveCurrentTokenPKI(ctx, name)
		require.NoError(t, err)
		assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
		assert.Contains(t, string(keyPEM), "BEGIN RSA PRIVATE KEY")
	})
}

func TestDEPStorage_SyncedDevices(t *testing.T) {
	database := setupTestDB(t)
	storage := NewDEPStorage(database.Writer, testEncKey)
	ctx := context.Background()
	name := "devices-test-" + time.Now().Format("150405")

	// Create dep_name row
	_, err := database.Writer.ExecContext(ctx, "INSERT INTO dep_names (name) VALUES ($1) ON CONFLICT DO NOTHING", name)
	require.NoError(t, err)
	t.Cleanup(func() {
		database.Writer.ExecContext(ctx, "DELETE FROM dep_devices WHERE dep_name = $1", name)
		database.Writer.ExecContext(ctx, "DELETE FROM dep_names WHERE name = $1", name)
	})

	t.Run("store and list", func(t *testing.T) {
		require.NoError(t, storage.StoreSyncedDevice(ctx, name, "SN001", map[string]interface{}{"model": "MacBook"}))
		require.NoError(t, storage.StoreSyncedDevice(ctx, name, "SN002", map[string]interface{}{"model": "iMac"}))

		devices, total, err := storage.ListDEPDevices(ctx, name, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, 2, len(devices))
	})

	t.Run("upsert updates existing device", func(t *testing.T) {
		require.NoError(t, storage.StoreSyncedDevice(ctx, name, "SN001", map[string]interface{}{"model": "MacBook Pro"}))

		devices, total, err := storage.ListDEPDevices(ctx, name, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, total) // still 2, not 3
		assert.Equal(t, 2, len(devices))
	})

	t.Run("empty list returns zero", func(t *testing.T) {
		devices, total, err := storage.ListDEPDevices(ctx, "nonexistent", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, devices)
	})
}
