package reporting

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceInventory(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))

	// Use a random enterprise ID — should return empty, not error
	rows, err := svc.DeviceInventory(context.Background(), uuid.New(), "")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestDeviceInventory_WithPlatformFilter(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))

	rows, err := svc.DeviceInventory(context.Background(), uuid.New(), "windows")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestComplianceReport(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))

	rows, err := svc.ComplianceReport(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestEnrollmentReport(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))

	rows, err := svc.EnrollmentReport(context.Background(), uuid.New(), 30)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestEnrollmentReport_DefaultDays(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))

	rows, err := svc.EnrollmentReport(context.Background(), uuid.New(), 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	err := WriteCSV(&buf, []string{"a", "b"}, [][]string{{"1", "2"}, {"3", "4"}})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "a,b")
	assert.Contains(t, buf.String(), "1,2")
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	err := WriteJSON(&buf, map[string]int{"count": 5})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"count": 5`)
}

func TestDeviceInventory_WithData(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))
	ctx := context.Background()

	entID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)`,
		entID, "report-test-"+entID.String()[:8], "rpt-"+entID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM enterprises WHERE id = $1", entID) })

	devID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO devices (id, enterprise_id, platform, device_id, serial_number, name, os_version, status, enrollment_date)
		 VALUES ($1, $2, 'macos', $3, 'SN001', 'MacBook', '14.0', 'enrolled', NOW())`,
		devID, entID, devID.String())
	require.NoError(t, err)

	rows, err := svc.DeviceInventory(ctx, entID, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "macos", rows[0].Platform)
	assert.Equal(t, "MacBook", rows[0].Name)
	assert.Equal(t, "SN001", rows[0].SerialNumber)

	// With platform filter
	rows, err = svc.DeviceInventory(ctx, entID, "macos")
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	rows, err = svc.DeviceInventory(ctx, entID, "windows")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestComplianceReport_WithData(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))
	ctx := context.Background()

	entID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)`,
		entID, "compliance-rpt-"+entID.String()[:8], "crpt-"+entID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM enterprises WHERE id = $1", entID) })

	devID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO devices (id, enterprise_id, platform, device_id, name, status, enrollment_date)
		 VALUES ($1, $2, 'windows', $3, 'Surface', 'enrolled', NOW())`,
		devID, entID, devID.String())
	require.NoError(t, err)

	polID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO policies (id, enterprise_id, name, platform, policy_type, policy_config, is_active)
		 VALUES ($1, $2, 'Security Policy', 'windows', 'security', '{}', true)`,
		polID, entID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO compliance_results (id, device_id, policy_id, status, details, evaluated_at)
		 VALUES ($1, $2, $3, 'compliant', '{"policy_type": "security", "violations": []}', NOW())`,
		uuid.New(), devID, polID)
	require.NoError(t, err)

	// Add a non-compliant result
	devID2 := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO devices (id, enterprise_id, platform, device_id, name, status, enrollment_date)
		 VALUES ($1, $2, 'macos', $3, 'MacBook', 'enrolled', NOW())`,
		devID2, entID, devID2.String())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO compliance_results (id, device_id, policy_id, status, details, evaluated_at)
		 VALUES ($1, $2, $3, 'non_compliant', '{"policy_type": "security", "violations": ["disk encryption not enabled", "firewall not enabled"]}', NOW())`,
		uuid.New(), devID2, polID)
	require.NoError(t, err)

	rows, err := svc.ComplianceReport(ctx, entID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// Compliant row
	compliant := rows[0]
	if compliant.Status != "compliant" {
		compliant = rows[1]
	}
	assert.Equal(t, "compliant", compliant.Status)
	assert.NotNil(t, compliant.Details)
	violations, ok := compliant.Details["violations"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, violations)

	// Non-compliant row
	nonCompliant := rows[1]
	if nonCompliant.Status != "non_compliant" {
		nonCompliant = rows[0]
	}
	assert.Equal(t, "non_compliant", nonCompliant.Status)
	assert.Equal(t, "security", nonCompliant.Details["policy_type"])
	ncViolations, ok := nonCompliant.Details["violations"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, ncViolations, 2)
	assert.Equal(t, "disk encryption not enabled", ncViolations[0])
}

func TestEnrollmentReport_WithData(t *testing.T) {
	db := testutil.ConnectRawDB(t)
	svc := NewService(NewSQLReportStore(db))
	ctx := context.Background()

	entID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)`,
		entID, "enroll-rpt-"+entID.String()[:8], "erpt-"+entID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM enterprises WHERE id = $1", entID) })

	for i := 0; i < 3; i++ {
		devID := uuid.New()
		_, err = db.ExecContext(ctx,
			`INSERT INTO devices (id, enterprise_id, platform, device_id, status, enrollment_date)
			 VALUES ($1, $2, 'macos', $3, 'enrolled', NOW())`,
			devID, entID, devID.String())
		require.NoError(t, err)
	}

	rows, err := svc.EnrollmentReport(ctx, entID, 7)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	total := 0
	for _, r := range rows {
		total += r.Count
	}
	assert.Equal(t, 3, total)
}

func TestWriteCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := WriteCSV(&buf, []string{"a"}, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "a")
}
