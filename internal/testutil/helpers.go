package testutil

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/require"
)

// CreateTestEnterprise inserts a test enterprise and registers t.Cleanup to delete it.
// The CASCADE on the enterprises FK will remove all child rows (devices, policies, etc.).
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
