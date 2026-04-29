package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredLogging(t *testing.T) {
	db := testutil.ConnectDB(t)

	ctx := context.Background()

	enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")

	// Create test user
	userID := uuid.New()
	_, err := db.Writer.ExecContext(ctx, `
		INSERT INTO users (id, enterprise_id, email, password_hash, role) 
		VALUES ($1, $2, $3, 'hash', 'admin')
	`, userID, enterpriseID, "test-"+userID.String()[:8]+"@example.com")
	require.NoError(t, err)

	t.Run("logs_successful_audit_event_with_structured_fields", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		auditLogger := NewLogger(db.Writer)
		auditLogger.SetLogger(logger)

		resourceID := uuid.New()
		err := auditLogger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			UserID:       userID,
			Action:       "device.create",
			ResourceType: "device",
			ResourceID:   resourceID,
			Details:      map[string]interface{}{"platform": "windows"},
			IPAddress:    net.ParseIP("192.168.1.100"),
			UserAgent:    "Test Agent",
		})
		require.NoError(t, err)

		// Parse log output
		logOutput := logBuf.String()
		t.Logf("Log output: %s", logOutput)

		// Verify structured fields are present
		assert.Contains(t, logOutput, "Audit event logged")
		assert.Contains(t, logOutput, enterpriseID.String())
		assert.Contains(t, logOutput, userID.String())
		assert.Contains(t, logOutput, "device.create")
		assert.Contains(t, logOutput, "device")
		assert.Contains(t, logOutput, resourceID.String())
		assert.Contains(t, logOutput, "192.168.1.100")

		// Verify it's valid JSON
		var logEntry map[string]interface{}
		err = json.Unmarshal([]byte(logOutput), &logEntry)
		require.NoError(t, err)

		// Verify structured fields
		assert.Equal(t, "Audit event logged", logEntry["msg"])
		assert.Equal(t, "INFO", logEntry["level"])
		assert.Equal(t, enterpriseID.String(), logEntry["enterprise_id"])
		assert.Equal(t, userID.String(), logEntry["user_id"])
		assert.Equal(t, "device.create", logEntry["action"])
		assert.Equal(t, "device", logEntry["resource_type"])
		assert.Equal(t, resourceID.String(), logEntry["resource_id"])
		assert.Equal(t, "192.168.1.100", logEntry["ip_address"])
	})

	t.Run("logs_error_with_structured_fields", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		auditLogger := NewLogger(db.Writer)
		auditLogger.SetLogger(logger)

		// Use invalid enterprise ID to trigger error
		invalidEnterpriseID := uuid.New()
		err := auditLogger.Log(ctx, Event{
			EnterpriseID: invalidEnterpriseID,
			UserID:       userID,
			Action:       "device.create",
			ResourceType: "device",
			ResourceID:   uuid.New(),
		})
		require.Error(t, err)

		// Parse log output
		logOutput := logBuf.String()
		t.Logf("Error log output: %s", logOutput)

		// Verify error is logged with structured fields
		assert.Contains(t, logOutput, "Failed to write audit log")
		assert.Contains(t, logOutput, invalidEnterpriseID.String())
		assert.Contains(t, logOutput, "device.create")

		// Verify it's valid JSON
		var logEntry map[string]interface{}
		err = json.Unmarshal([]byte(logOutput), &logEntry)
		require.NoError(t, err)

		// Verify structured fields
		assert.Equal(t, "Failed to write audit log", logEntry["msg"])
		assert.Equal(t, "ERROR", logEntry["level"])
		assert.Equal(t, invalidEnterpriseID.String(), logEntry["enterprise_id"])
		assert.Equal(t, "device.create", logEntry["action"])
		assert.NotNil(t, logEntry["error"])
	})

	t.Run("works_without_logger_set", func(t *testing.T) {
		auditLogger := NewLogger(db.Writer)
		// Don't set logger - should use default

		err := auditLogger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			UserID:       userID,
			Action:       "device.update",
			ResourceType: "device",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	})

	t.Run("handles_nil_logger_gracefully", func(t *testing.T) {
		auditLogger := NewLogger(db.Writer)
		auditLogger.SetLogger(nil) // Should not panic

		err := auditLogger.Log(ctx, Event{
			EnterpriseID: enterpriseID,
			UserID:       userID,
			Action:       "device.delete",
			ResourceType: "device",
			ResourceID:   uuid.New(),
		})
		require.NoError(t, err)
	})

	t.Run("logs_all_event_types", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		auditLogger := NewLogger(db.Writer)
		auditLogger.SetLogger(logger)

		actions := []string{
			"device.create",
			"device.update",
			"device.delete",
			"policy.create",
			"policy.update",
			"user.login",
			"user.logout",
		}

		for _, action := range actions {
			err := auditLogger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				UserID:       userID,
				Action:       action,
				ResourceType: strings.Split(action, ".")[0],
				ResourceID:   uuid.New(),
			})
			require.NoError(t, err)
		}

		logOutput := logBuf.String()
		
		// Verify all actions are logged
		for _, action := range actions {
			assert.Contains(t, logOutput, action)
		}
	})

	t.Run("structured_fields_are_searchable", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		auditLogger := NewLogger(db.Writer)
		auditLogger.SetLogger(logger)

		// Log multiple events
		for i := 0; i < 5; i++ {
			err := auditLogger.Log(ctx, Event{
				EnterpriseID: enterpriseID,
				Action:       "device.create",
				ResourceType: "device",
				ResourceID:   uuid.New(),
			})
			require.NoError(t, err)
		}

		logOutput := logBuf.String()
		lines := strings.Split(strings.TrimSpace(logOutput), "\n")
		
		// Each line should be valid JSON
		for _, line := range lines {
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(line), &logEntry)
			require.NoError(t, err, "Each log line should be valid JSON")
			
			// Verify searchable fields exist
			assert.NotNil(t, logEntry["enterprise_id"])
			assert.NotNil(t, logEntry["action"])
			assert.NotNil(t, logEntry["resource_type"])
			assert.NotNil(t, logEntry["resource_id"])
			assert.NotNil(t, logEntry["time"])
		}
	})
}
