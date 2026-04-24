package testutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
)

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
