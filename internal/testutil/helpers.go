package testutil

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/require"
)

// TestEnterpriseID is the well-known UUID for the dedicated test enterprise.
// This enterprise is created by seed_data.sql and must never be deleted.
// All integration tests should use this instead of creating ad-hoc enterprises.
var TestEnterpriseID = uuid.MustParse("99999999-9999-9999-9999-999999999999")

// EnsureTestEnterprise verifies the test enterprise exists, creating it if needed (idempotent).
// Returns TestEnterpriseID. Does NOT register cleanup — the test enterprise persists across runs.
func EnsureTestEnterprise(t testing.TB, db *sql.DB) uuid.UUID {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3) ON CONFLICT (slug) DO NOTHING`,
		TestEnterpriseID, "Test Enterprise (DO NOT DELETE)", "test-enterprise",
	)
	require.NoError(t, err)
	return TestEnterpriseID
}

// CleanupTestData deletes all child rows under the test enterprise (devices, policies,
// groups, tokens, etc.) but never deletes the enterprise itself. Safe to call in t.Cleanup().
// Deletes in FK-safe order: tables with enterprise_id first, then cascade-dependent tables
// are handled automatically by the ON DELETE CASCADE constraints.
func CleanupTestData(t testing.TB, db *sql.DB) {
	t.Helper()
	// Tables with direct enterprise_id FK — delete in child-first order.
	tables := []string{
		"enrollment_tokens",
		"audit_logs",
		"devices",
		"policies",
		"device_groups",
		"users",
	}
	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE enterprise_id = $1", table), TestEnterpriseID)
		if err != nil {
			t.Logf("CleanupTestData: %s: %v", table, err)
		}
	}
}

// CreateTestEnterprise inserts a test enterprise and registers t.Cleanup to delete it.
// The CASCADE on the enterprises FK will remove all child rows (devices, policies, etc.).
// Deprecated: prefer EnsureTestEnterprise + TestEnterpriseID for new tests.
func CreateTestEnterprise(t testing.TB, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	slug := fmt.Sprintf("test-%s", id.String()[:8])
	_, err := db.Exec("INSERT INTO enterprises (id, name, slug) VALUES ($1, $2, $3)", id, name, slug)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM enterprises WHERE id = $1", id)
	})
	return id
}

// NewTestEnterprise creates a test enterprise with random slug
func NewTestEnterprise(t *testing.T) *models.Enterprise {
	t.Helper()
	return &models.Enterprise{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		Name: "Test Enterprise",
		Slug: "test-" + uuid.New().String()[:8],
		Settings: models.JSONB{
			"test": true,
		},
	}
}

// NewTestDevice creates a test device
func NewTestDevice(t *testing.T, enterpriseID uuid.UUID) *models.Device {
	t.Helper()
	return &models.Device{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		EnterpriseID: enterpriseID,
		Platform:     "macos",
		DeviceID:     "test-device-" + uuid.New().String(),
		SerialNumber: "SN" + uuid.New().String()[:8],
		Name:         "Test Device",
		Model:        "MacBookPro18,1",
		OSVersion:    "14.0",
		Status:       "enrolled",
		PlatformData: models.JSONB{"test": true},
	}
}

// NewTestPolicy creates a test policy
func NewTestPolicy(t *testing.T, enterpriseID uuid.UUID) *models.Policy {
	t.Helper()
	return &models.Policy{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		EnterpriseID: enterpriseID,
		Name:         "Test Policy",
		Description:  "Test policy description",
		Platform:     "macos",
		PolicyType:   "wifi",
		PolicyConfig: models.JSONB{
			"ssid": "TestNetwork",
		},
		IsActive: true,
	}
}
