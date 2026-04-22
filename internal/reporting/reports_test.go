package reporting

import (
	"bytes"
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=localmdm sslmode=disable")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping: %v", err)
	}
	db.SetMaxOpenConns(2)
	return db
}

func TestDeviceInventory(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	svc := NewService(db)

	// Use a random enterprise ID — should return empty, not error
	rows, err := svc.DeviceInventory(context.Background(), uuid.New(), "")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestDeviceInventory_WithPlatformFilter(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	svc := NewService(db)

	rows, err := svc.DeviceInventory(context.Background(), uuid.New(), "windows")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestComplianceReport(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	svc := NewService(db)

	rows, err := svc.ComplianceReport(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestEnrollmentReport(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	svc := NewService(db)

	rows, err := svc.EnrollmentReport(context.Background(), uuid.New(), 30)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestEnrollmentReport_DefaultDays(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	svc := NewService(db)

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
