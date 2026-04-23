package certs

import (
	"bytes"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/db"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func connectAndCleanDB(t testing.TB) *db.DB {
	t.Helper()
	database := testutil.ConnectDB(t)
	_, err := database.Writer.Exec("DELETE FROM certificates")
	if err != nil {
		t.Fatalf("Failed to clean certificates: %v", err)
	}
	_, err = database.Writer.Exec("DELETE FROM devices")
	if err != nil {
		t.Fatalf("Failed to clean devices: %v", err)
	}
	return database
}

func createTestDevice(t testing.TB, db *sql.DB, enterpriseID uuid.UUID) uuid.UUID {
	t.Helper()

	deviceID := uuid.New()
	query := `
		INSERT INTO devices (id, enterprise_id, platform, device_id, name, status)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := db.Exec(query, deviceID, enterpriseID, "ios", "test-device-"+deviceID.String(), "Test Device", "enrolled")
	require.NoError(t, err)

	return deviceID
}

func createTestCertificateWithDevice(t testing.TB, db *sql.DB, expiresAt time.Time, revoked bool) uuid.UUID {
	t.Helper()

	// Create enterprise first
	enterpriseID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO enterprises (id, name, slug)
		VALUES ($1, $2, $3)
	`, enterpriseID, "Test Enterprise", "test-"+enterpriseID.String())
	require.NoError(t, err)

	// Create device
	deviceID := createTestDevice(t, db, enterpriseID)

	// Create certificate
	return createTestCertificate(t, db, deviceID, expiresAt, revoked)
}

func createTestCertificate(t testing.TB, db *sql.DB, deviceID uuid.UUID, expiresAt time.Time, revoked bool) uuid.UUID {
	t.Helper()

	certID := uuid.New()
	
	query := `
		INSERT INTO certificates (id, device_id, cert_type, subject, serial_number, cert_data, issued_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	var revokedAt *time.Time
	if revoked {
		now := time.Now()
		revokedAt = &now
	}

	_, err := db.Exec(query,
		certID,
		deviceID,
		"device",
		"CN=test-device",
		certID.String(),
		"-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		time.Now().Add(-365*24*time.Hour),
		expiresAt,
		revokedAt,
	)
	require.NoError(t, err)

	return certID
}

// TestExpirationMonitor_Start tests that monitor starts successfully
func TestExpirationMonitor_Start(t *testing.T) {
	database := connectAndCleanDB(t)

	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	defer monitor.Stop()

	// Verify monitor is running
	monitor.mu.Lock()
	running := monitor.running
	monitor.mu.Unlock()

	assert.True(t, running, "Monitor should be running")
}

// TestExpirationMonitor_Stop tests graceful shutdown
func TestExpirationMonitor_Stop(t *testing.T) {
	database := connectAndCleanDB(t)

	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(50 * time.Millisecond) // Let it start
	
	monitor.Stop()

	// Verify monitor stopped
	monitor.mu.Lock()
	running := monitor.running
	monitor.mu.Unlock()

	assert.False(t, running, "Monitor should be stopped")
}

// TestExpirationMonitor_IdempotentStart tests multiple Start calls
func TestExpirationMonitor_IdempotentStart(t *testing.T) {
	database := connectAndCleanDB(t)

	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	monitor.Start() // Second start should be no-op
	monitor.Start() // Third start should be no-op
	
	defer monitor.Stop()

	// Should not panic or cause issues
	monitor.mu.Lock()
	running := monitor.running
	monitor.mu.Unlock()

	assert.True(t, running)
}

// TestExpirationMonitor_IdempotentStop tests multiple Stop calls
func TestExpirationMonitor_IdempotentStop(t *testing.T) {
	database := connectAndCleanDB(t)

	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	monitor.Stop()
	monitor.Stop() // Second stop should be no-op
	monitor.Stop() // Third stop should be no-op

	// Should not panic
	monitor.mu.Lock()
	running := monitor.running
	monitor.mu.Unlock()

	assert.False(t, running)
}

// TestExpirationMonitor_DetectsExpiringCertificates tests warning for expiring certs
func TestExpirationMonitor_DetectsExpiringCertificates(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create certificate expiring in 15 days
	expiresAt := time.Now().Add(15 * 24 * time.Hour)
	certID := createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify warning was logged
	logs := logBuf.String()
	assert.Contains(t, logs, "Certificate expiring soon")
	assert.Contains(t, logs, certID.String())
	assert.Contains(t, logs, "days_remaining")
}

// TestExpirationMonitor_IgnoresNonExpiringCertificates tests no warning for valid certs
func TestExpirationMonitor_IgnoresNonExpiringCertificates(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create certificate expiring in 60 days (beyond threshold)
	expiresAt := time.Now().Add(60 * 24 * time.Hour)
	createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify no warning was logged
	logs := logBuf.String()
	assert.NotContains(t, logs, "Certificate expiring soon")
}

// TestExpirationMonitor_IgnoresRevokedCertificates tests revoked certs are ignored
func TestExpirationMonitor_IgnoresRevokedCertificates(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create revoked certificate expiring in 15 days
	expiresAt := time.Now().Add(15 * 24 * time.Hour)
	certID := createTestCertificateWithDevice(t, database.Writer, expiresAt, true)

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify no warning for revoked cert
	logs := logBuf.String()
	assert.NotContains(t, logs, certID.String())
}

// TestExpirationMonitor_IgnoresExpiredCertificates tests already expired certs
func TestExpirationMonitor_IgnoresExpiredCertificates(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create already expired certificate
	expiresAt := time.Now().Add(-1 * 24 * time.Hour)
	certID := createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify no warning for already expired cert
	logs := logBuf.String()
	assert.NotContains(t, logs, certID.String())
}

// TestExpirationMonitor_MultipleExpiringCertificates tests multiple warnings
func TestExpirationMonitor_MultipleExpiringCertificates(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create 3 expiring certificates
	cert1 := createTestCertificateWithDevice(t, database.Writer, time.Now().Add(5*24*time.Hour), false)
	cert2 := createTestCertificateWithDevice(t, database.Writer, time.Now().Add(15*24*time.Hour), false)
	cert3 := createTestCertificateWithDevice(t, database.Writer, time.Now().Add(25*24*time.Hour), false)

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify all 3 certificates logged
	logs := logBuf.String()
	assert.Contains(t, logs, cert1.String())
	assert.Contains(t, logs, cert2.String())
	assert.Contains(t, logs, cert3.String())
	assert.Contains(t, logs, "expiring_count\":3")
}

// TestExpirationMonitor_CustomThreshold tests custom warning threshold
func TestExpirationMonitor_CustomThreshold(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create certificate expiring in 5 days
	expiresAt := time.Now().Add(5 * 24 * time.Hour)
	certID := createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	// Use 7-day threshold (should warn since 5 < 7)
	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 7*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify warning was logged
	logs := logBuf.String()
	assert.Contains(t, logs, certID.String())
}

// TestExpirationMonitor_PeriodicChecks tests multiple periodic checks
func TestExpirationMonitor_PeriodicChecks(t *testing.T) {
	database := connectAndCleanDB(t)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create expiring certificate
	expiresAt := time.Now().Add(15 * 24 * time.Hour)
	createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	// Check every 50ms
	monitor := NewExpirationMonitor(database.Writer, logger, 50*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(200 * time.Millisecond) // Wait for multiple checks
	monitor.Stop()

	// Verify multiple checks occurred
	logs := logBuf.String()
	count := bytes.Count([]byte(logs), []byte("Certificate expiration check completed"))
	assert.GreaterOrEqual(t, count, 3, "Should have multiple periodic checks")
}

// TestExpirationMonitor_DatabaseError tests error handling
func TestExpirationMonitor_DatabaseError(t *testing.T) {
	database := connectAndCleanDB(t)
	database.Close() // Close database to cause error

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	monitor := NewExpirationMonitor(database.Writer, logger, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Verify error was logged
	logs := logBuf.String()
	assert.Contains(t, logs, "Failed to check expiring certificates")
}

// TestExpirationMonitor_DefaultValues tests default configuration
func TestExpirationMonitor_DefaultValues(t *testing.T) {
	database := connectAndCleanDB(t)

	// Pass zero values to trigger defaults
	monitor := NewExpirationMonitor(database.Writer, nil, 0, 0)

	assert.Equal(t, 24*time.Hour, monitor.checkInterval, "Should use default check interval")
	assert.Equal(t, 30*24*time.Hour, monitor.warningThreshold, "Should use default warning threshold")
}

// TestExpirationMonitor_NilLogger tests operation without logger
func TestExpirationMonitor_NilLogger(t *testing.T) {
	database := connectAndCleanDB(t)

	// Create expiring certificate
	expiresAt := time.Now().Add(15 * 24 * time.Hour)
	createTestCertificateWithDevice(t, database.Writer, expiresAt, false)

	// No logger provided
	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)
	
	monitor.Start()
	time.Sleep(150 * time.Millisecond) // Wait for check
	monitor.Stop()

	// Should not panic
}

// TestExpirationMonitor_ConcurrentStartStop tests concurrent operations
func TestExpirationMonitor_ConcurrentStartStop(t *testing.T) {
	database := connectAndCleanDB(t)

	monitor := NewExpirationMonitor(database.Writer, nil, 100*time.Millisecond, 30*24*time.Hour)

	// Start and stop concurrently from multiple goroutines
	done := make(chan bool, 10)
	
	// Multiple starts (only first should succeed)
	for i := 0; i < 5; i++ {
		go func() {
			monitor.Start()
			done <- true
		}()
	}
	
	// Wait for starts
	for i := 0; i < 5; i++ {
		<-done
	}
	
	// Multiple stops (only first should do work)
	for i := 0; i < 5; i++ {
		go func() {
			monitor.Stop()
			done <- true
		}()
	}

	// Wait for stops
	for i := 0; i < 5; i++ {
		<-done
	}

	// Should not panic or deadlock
}

// BenchmarkExpirationMonitor_Check benchmarks certificate checking
func BenchmarkExpirationMonitor_Check(b *testing.B) {
	database := connectAndCleanDB(b)

	// Create 100 expiring certificates
	for i := 0; i < 100; i++ {
		expiresAt := time.Now().Add(time.Duration(i) * 24 * time.Hour)
		createTestCertificateWithDevice(b, database.Writer, expiresAt, false)
	}

	monitor := NewExpirationMonitor(database.Writer, nil, 24*time.Hour, 30*24*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.checkExpiringCertificates()
	}
}
