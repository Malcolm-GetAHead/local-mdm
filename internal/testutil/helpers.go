package testutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/require"
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

// AssertNoError is a helper that fails the test if err is not nil
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	require.NoError(t, err, msgAndArgs...)
}

// AssertError is a helper that fails the test if err is nil
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	require.Error(t, err, msgAndArgs...)
}

// AssertEqual is a helper for equality assertions
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected, actual, msgAndArgs...)
}

// AssertNotNil is a helper that fails if value is nil
func AssertNotNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.NotNil(t, value, msgAndArgs...)
}
