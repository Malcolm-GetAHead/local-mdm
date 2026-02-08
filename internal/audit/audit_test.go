package audit

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
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
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	return database
}

func TestLogger_Log_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	resourceID := uuid.New()

	event := Event{
		Action:       "device.create",
		ResourceType: "device",
		ResourceID:   resourceID,
		Details: map[string]interface{}{
			"platform": "ios",
			"model":    "iPhone 14",
		},
		IPAddress: net.ParseIP("192.168.1.100"),
		UserAgent: "Mozilla/5.0",
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify the log was written (use resource_id for uniqueness)
	var count int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1", resourceID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the details
	var (
		loggedAction       string
		loggedResourceType string
		loggedIPAddress    string
	)
	err = database.QueryRowContext(ctx,
		"SELECT action, resource_type, ip_address FROM audit_logs WHERE action = $1 ORDER BY created_at DESC LIMIT 1",
		"device.create",
	).Scan(&loggedAction, &loggedResourceType, &loggedIPAddress)
	require.NoError(t, err)
	assert.Equal(t, "device.create", loggedAction)
	assert.Equal(t, "device", loggedResourceType)
	assert.Equal(t, "192.168.1.100", loggedIPAddress)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "device.create")
}

func TestLogger_Log_MinimalEvent(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		Action:       "auth.login",
		ResourceType: "user",
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify the log was written with NULL values
	var (
		enterpriseID *uuid.UUID
		userID       *uuid.UUID
		resourceID   *uuid.UUID
		ipAddress    *string
		userAgent    *string
	)
	err = database.QueryRowContext(ctx,
		"SELECT enterprise_id, user_id, resource_id, ip_address, user_agent FROM audit_logs WHERE action = $1 ORDER BY created_at DESC LIMIT 1",
		"auth.login",
	).Scan(&enterpriseID, &userID, &resourceID, &ipAddress, &userAgent)
	require.NoError(t, err)
	assert.Nil(t, enterpriseID)
	assert.Nil(t, userID)
	assert.Nil(t, resourceID)
	assert.Nil(t, ipAddress)
	assert.Nil(t, userAgent)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "auth.login")
}

func TestLogger_Log_InvalidAction(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	tests := []struct {
		name  string
		event Event
		errMsg string
	}{
		{
			name: "empty action",
			event: Event{
				Action:       "",
				ResourceType: "device",
			},
			errMsg: "action is required",
		},
		{
			name: "empty resource type",
			event: Event{
				Action:       "device.create",
				ResourceType: "",
			},
			errMsg: "resource_type is required",
		},
		{
			name: "action too long",
			event: Event{
				Action:       string(make([]byte, 101)),
				ResourceType: "device",
			},
			errMsg: "action exceeds 100 characters",
		},
		{
			name: "resource type too long",
			event: Event{
				Action:       "device.create",
				ResourceType: string(make([]byte, 51)),
			},
			errMsg: "resource_type exceeds 50 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.Log(ctx, tt.event)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestLogger_Log_WithNilUUIDs(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		EnterpriseID: uuid.Nil,
		UserID:       uuid.Nil,
		Action:       "system.startup",
		ResourceType: "system",
		ResourceID:   uuid.Nil,
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "system.startup")
}

func TestLogger_Log_WithIPv6(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		Action:       "device.update",
		ResourceType: "device",
		IPAddress:    net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify IPv6 was stored
	var ipAddress string
	err = database.QueryRowContext(ctx,
		"SELECT ip_address FROM audit_logs WHERE action = $1 ORDER BY created_at DESC LIMIT 1",
		"device.update",
	).Scan(&ipAddress)
	require.NoError(t, err)
	assert.Contains(t, ipAddress, "2001:db8:85a3")

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "device.update")
}

func TestLogger_Log_WithComplexDetails(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		Action:       "policy.update",
		ResourceType: "policy",
		Details: map[string]interface{}{
			"old_value": map[string]interface{}{
				"enabled": false,
				"rules":   []string{"rule1", "rule2"},
			},
			"new_value": map[string]interface{}{
				"enabled": true,
				"rules":   []string{"rule1", "rule2", "rule3"},
			},
			"changed_by": "admin@example.com",
		},
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify details were stored as JSONB
	var details string
	err = database.QueryRowContext(ctx,
		"SELECT details::text FROM audit_logs WHERE action = $1 ORDER BY created_at DESC LIMIT 1",
		"policy.update",
	).Scan(&details)
	require.NoError(t, err)
	assert.Contains(t, details, "old_value")
	assert.Contains(t, details, "new_value")
	assert.Contains(t, details, "changed_by")

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "policy.update")
}

func TestLogger_Log_ConcurrentWrites(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	// Write 10 events concurrently
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			event := Event{
				Action:       "concurrent.test",
				ResourceType: "test",
				Details: map[string]interface{}{
					"iteration": n,
				},
			}
			done <- logger.Log(ctx, event)
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		err := <-done
		require.NoError(t, err)
	}

	// Verify all 10 were written
	var count int
	err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action = $1", "concurrent.test").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "concurrent.test")
}

func TestLogger_Log_ContextCancellation(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := Event{
		Action:       "cancelled.test",
		ResourceType: "test",
	}

	err := logger.Log(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestLogger_Log_EmptyDetails(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		Action:       "empty.details",
		ResourceType: "test",
		Details:      map[string]interface{}{},
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify empty details stored as {}
	var details string
	err = database.QueryRowContext(ctx,
		"SELECT details::text FROM audit_logs WHERE action = $1 ORDER BY created_at DESC LIMIT 1",
		"empty.details",
	).Scan(&details)
	require.NoError(t, err)
	assert.Equal(t, "{}", details)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "empty.details")
}

func TestLogger_Log_NilDetails(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	logger := NewLogger(database.DB)
	ctx := context.Background()

	event := Event{
		Action:       "nil.details",
		ResourceType: "test",
		Details:      nil,
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Cleanup
	_, _ = database.ExecContext(ctx, "DELETE FROM audit_logs WHERE action = $1", "nil.details")
}

func TestValidateEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: Event{
				Action:       "test.action",
				ResourceType: "test",
			},
			wantErr: false,
		},
		{
			name: "missing action",
			event: Event{
				ResourceType: "test",
			},
			wantErr: true,
			errMsg:  "action is required",
		},
		{
			name: "missing resource type",
			event: Event{
				Action: "test.action",
			},
			wantErr: true,
			errMsg:  "resource_type is required",
		},
		{
			name: "action too long",
			event: Event{
				Action:       string(make([]byte, 101)),
				ResourceType: "test",
			},
			wantErr: true,
			errMsg:  "action exceeds 100 characters",
		},
		{
			name: "resource type too long",
			event: Event{
				Action:       "test.action",
				ResourceType: string(make([]byte, 51)),
			},
			wantErr: true,
			errMsg:  "resource_type exceeds 50 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEvent(tt.event)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
