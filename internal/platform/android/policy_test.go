package android

import (
	"encoding/json"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslatePolicy_Security(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"min_password_length":  float64(8),
			"password_quality":     "ALPHANUMERIC",
			"max_failed_attempts":  float64(10),
			"require_encryption":   true,
			"lock_timeout_minutes": float64(5),
		},
	}

	ap := TranslatePolicy(policy)
	require.NotNil(t, ap.PasswordRequirements)
	assert.Equal(t, int64(8), ap.PasswordRequirements.PasswordMinimumLength)
	assert.Equal(t, "ALPHANUMERIC", ap.PasswordRequirements.PasswordQuality)
	assert.Equal(t, int64(10), ap.PasswordRequirements.MaximumFailedPasswordsForWipe)
	assert.Equal(t, "ENABLED_WITHOUT_PASSWORD", ap.EncryptionPolicy)
	assert.Equal(t, int64(300000), ap.MaximumTimeToLock) // 5 min in ms
}

func TestTranslatePolicy_Restrictions(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeRestriction,
		PolicyConfig: models.JSONB{
			"allow_camera":          false,
			"allow_screen_capture":  false,
			"allow_usb_transfer":    false,
			"allow_bluetooth":       false,
		},
	}

	ap := TranslatePolicy(policy)
	assert.True(t, ap.CameraDisabled)
	assert.True(t, ap.ScreenCaptureDisabled)
	assert.True(t, ap.UsbFileTransferDisabled)
	assert.True(t, ap.BluetoothDisabled)
}

func TestTranslatePolicy_Restrictions_Allowed(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeRestriction,
		PolicyConfig: models.JSONB{
			"allow_camera": true,
		},
	}

	ap := TranslatePolicy(policy)
	assert.False(t, ap.CameraDisabled) // camera allowed = not disabled
}

func TestTranslatePolicy_WiFi(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid":          "CorpNet",
			"security_type": "WPA2_PSK",
			"password":      "secret123",
		},
	}

	ap := TranslatePolicy(policy)
	require.NotNil(t, ap.OpenNetworkConfiguration)

	var onc map[string]interface{}
	err := json.Unmarshal(ap.OpenNetworkConfiguration, &onc)
	require.NoError(t, err)
	configs, ok := onc["NetworkConfigurations"].([]interface{})
	require.True(t, ok)
	require.Len(t, configs, 1)
	cfg := configs[0].(map[string]interface{})
	assert.Equal(t, "CorpNet", cfg["Name"])
}

func TestTranslatePolicy_App(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeApp,
		PolicyConfig: models.JSONB{
			"applications": []interface{}{
				map[string]interface{}{
					"package_name": "com.example.app",
					"install_type": "FORCE_INSTALLED",
				},
				map[string]interface{}{
					"package_name": "com.example.optional",
					"install_type": "AVAILABLE",
				},
			},
		},
	}

	ap := TranslatePolicy(policy)
	require.Len(t, ap.Applications, 2)
	assert.Equal(t, "com.example.app", ap.Applications[0].PackageName)
	assert.Equal(t, "FORCE_INSTALLED", ap.Applications[0].InstallType)
	assert.Equal(t, "com.example.optional", ap.Applications[1].PackageName)
	assert.Equal(t, "AVAILABLE", ap.Applications[1].InstallType)
}

func TestTranslatePolicy_EmptyConfig(t *testing.T) {
	policy := &models.Policy{
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: nil,
	}

	ap := TranslatePolicy(policy)
	assert.Nil(t, ap.PasswordRequirements)
}

func TestTranslatePolicy_EmptyAppList(t *testing.T) {
	policy := &models.Policy{
		PolicyType:   models.PolicyTypeApp,
		PolicyConfig: models.JSONB{},
	}

	ap := TranslatePolicy(policy)
	assert.Nil(t, ap.Applications)
}

func TestTranslatePolicy_SecurityPartial(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeSecurity,
		PolicyConfig: models.JSONB{
			"min_password_length": float64(6),
		},
	}

	ap := TranslatePolicy(policy)
	require.NotNil(t, ap.PasswordRequirements)
	assert.Equal(t, int64(6), ap.PasswordRequirements.PasswordMinimumLength)
	assert.Equal(t, "", ap.EncryptionPolicy) // not set
}
