package audit

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_Log_Success(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")
	resourceID := uuid.New()

	event := Event{
		EnterpriseID: enterpriseID,
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
	err = database.Writer.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE resource_id = $1", resourceID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the details
	var (
		loggedAction       string
		loggedResourceType string
		loggedIPAddress    string
	)
	err = database.Writer.QueryRowContext(ctx,
		"SELECT action, resource_type, ip_address FROM audit_logs WHERE resource_id = $1",
		resourceID,
	).Scan(&loggedAction, &loggedResourceType, &loggedIPAddress)
	require.NoError(t, err)
	assert.Equal(t, "device.create", loggedAction)
	assert.Equal(t, "device", loggedResourceType)
	assert.Equal(t, "192.168.1.100", loggedIPAddress)
}

func TestLogger_Log_MinimalEvent(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")
	resourceID := uuid.New()

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "auth.login",
		ResourceType: "user",
		ResourceID:   resourceID,
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify the log was written with expected NULL values
	var (
		userID    *uuid.UUID
		ipAddress *string
		userAgent *string
	)
	err = database.Writer.QueryRowContext(ctx,
		"SELECT user_id, ip_address, user_agent FROM audit_logs WHERE resource_id = $1",
		resourceID,
	).Scan(&userID, &ipAddress, &userAgent)
	require.NoError(t, err)
	assert.Nil(t, userID)
	assert.Nil(t, ipAddress)
	assert.Nil(t, userAgent)
}

func TestLogger_Log_InvalidAction(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
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
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "system.startup",
		ResourceType: "system",
		ResourceID:   uuid.Nil,
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)
}

func TestLogger_Log_WithIPv6(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")
	resourceID := uuid.New()

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "device.update",
		ResourceType: "device",
		ResourceID:   resourceID,
		IPAddress:    net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify IPv6 was stored
	var ipAddress string
	err = database.Writer.QueryRowContext(ctx,
		"SELECT ip_address FROM audit_logs WHERE resource_id = $1",
		resourceID,
	).Scan(&ipAddress)
	require.NoError(t, err)
	assert.Contains(t, ipAddress, "2001:db8:85a3")
}

func TestLogger_Log_WithComplexDetails(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")
	resourceID := uuid.New()

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "policy.update",
		ResourceType: "policy",
		ResourceID:   resourceID,
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
	err = database.Writer.QueryRowContext(ctx,
		"SELECT details::text FROM audit_logs WHERE resource_id = $1",
		resourceID,
	).Scan(&details)
	require.NoError(t, err)
	assert.Contains(t, details, "old_value")
	assert.Contains(t, details, "new_value")
	assert.Contains(t, details, "changed_by")
}

func TestLogger_Log_ConcurrentWrites(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")

	// Write 10 events concurrently
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			event := Event{
				EnterpriseID: enterpriseID,
				Action:       "sync.concurrent.test",
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
	err := database.Writer.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action = $1 AND enterprise_id = $2", "sync.concurrent.test", enterpriseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count)
}

func TestLogger_Log_ContextCancellation(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
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
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")
	resourceID := uuid.New()

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "empty.details",
		ResourceType: "test",
		ResourceID:   resourceID,
		Details:      map[string]interface{}{},
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)

	// Verify empty details stored as {}
	var details string
	err = database.Writer.QueryRowContext(ctx,
		"SELECT details::text FROM audit_logs WHERE resource_id = $1",
		resourceID,
	).Scan(&details)
	require.NoError(t, err)
	assert.Equal(t, "{}", details)
}

func TestLogger_Log_NilDetails(t *testing.T) {
	database := testutil.ConnectDB(t)

	logger := NewLogger(database.Writer)
	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, database.Writer, "Test Enterprise")

	event := Event{
		EnterpriseID: enterpriseID,
		Action:       "nil.details",
		ResourceType: "test",
		Details:      nil,
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)
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
