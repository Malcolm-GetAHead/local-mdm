package android

import (
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestTranslatePolicy_NilConfig(t *testing.T) {
	policy := &models.Policy{
		PolicyType:   models.PolicyTypeSecurity,
		PolicyConfig: nil,
	}
	ap := TranslatePolicy(policy)
	assert.NotNil(t, ap)
	assert.Nil(t, ap.PasswordRequirements)
}

func TestApplySecurityPolicy(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		policy := &models.Policy{
			PolicyType: models.PolicyTypeSecurity,
			PolicyConfig: models.JSONB{
				"min_password_length": float64(8),
				"password_quality":    "NUMERIC_COMPLEX",
				"max_failed_attempts": float64(5),
				"require_encryption":  true,
				"lock_timeout_minutes": float64(10),
			},
		}
		ap := TranslatePolicy(policy)
		assert.NotNil(t, ap.PasswordRequirements)
		assert.Equal(t, int64(8), ap.PasswordRequirements.PasswordMinimumLength)
		assert.Equal(t, "NUMERIC_COMPLEX", ap.PasswordRequirements.PasswordQuality)
		assert.Equal(t, int64(5), ap.PasswordRequirements.MaximumFailedPasswordsForWipe)
		assert.Equal(t, "ENABLED_WITHOUT_PASSWORD", ap.EncryptionPolicy)
		assert.Equal(t, int64(600000), ap.MaximumTimeToLock)
	})

	t.Run("no password requirements", func(t *testing.T) {
		policy := &models.Policy{
			PolicyType: models.PolicyTypeSecurity,
			PolicyConfig: models.JSONB{
				"require_encryption": false,
			},
		}
		ap := TranslatePolicy(policy)
		assert.Nil(t, ap.PasswordRequirements)
		assert.Empty(t, ap.EncryptionPolicy)
	})
}

func TestApplyRestrictionPolicy(t *testing.T) {
	t.Run("all restrictions disabled", func(t *testing.T) {
		policy := &models.Policy{
			PolicyType: models.PolicyTypeRestriction,
			PolicyConfig: models.JSONB{
				"allow_camera":         false,
				"allow_screen_capture": false,
				"allow_usb_transfer":   false,
				"allow_bluetooth":      false,
			},
		}
		ap := TranslatePolicy(policy)
		assert.True(t, ap.CameraDisabled)
		assert.True(t, ap.ScreenCaptureDisabled)
		assert.True(t, ap.UsbFileTransferDisabled)
		assert.True(t, ap.BluetoothDisabled)
	})

	t.Run("all restrictions allowed", func(t *testing.T) {
		policy := &models.Policy{
			PolicyType: models.PolicyTypeRestriction,
			PolicyConfig: models.JSONB{
				"allow_camera":         true,
				"allow_screen_capture": true,
				"allow_usb_transfer":   true,
				"allow_bluetooth":      true,
			},
		}
		ap := TranslatePolicy(policy)
		assert.False(t, ap.CameraDisabled)
		assert.False(t, ap.ScreenCaptureDisabled)
		assert.False(t, ap.UsbFileTransferDisabled)
		assert.False(t, ap.BluetoothDisabled)
	})
}

func TestApplyWiFiPolicy_SSIDOnly(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid": "OpenNetwork",
			// no password, no security_type
		},
	}
	ap := TranslatePolicy(policy)
	assert.NotNil(t, ap.OpenNetworkConfiguration)
}

func TestApplyWiFiPolicy_EmptySSID(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeWiFi,
		PolicyConfig: models.JSONB{
			"ssid": "",
		},
	}
	ap := TranslatePolicy(policy)
	assert.Nil(t, ap.OpenNetworkConfiguration)
}

func TestApplyAppPolicy_EmptyApps(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeApp,
		PolicyConfig: models.JSONB{
			"applications": []interface{}{},
		},
	}
	ap := TranslatePolicy(policy)
	assert.Nil(t, ap.Applications)
}

func TestApplyAppPolicy_InvalidAppEntry(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeApp,
		PolicyConfig: models.JSONB{
			"applications": []interface{}{
				"not-a-map",
				map[string]interface{}{"package_name": ""},
				map[string]interface{}{"package_name": "com.valid.app"},
			},
		},
	}
	ap := TranslatePolicy(policy)
	assert.Len(t, ap.Applications, 1)
	assert.Equal(t, "com.valid.app", ap.Applications[0].PackageName)
}

func TestApplyAppPolicy_CustomInstallType(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeApp,
		PolicyConfig: models.JSONB{
			"applications": []interface{}{
				map[string]interface{}{
					"package_name": "com.example.app",
					"install_type": "AVAILABLE",
				},
			},
		},
	}
	ap := TranslatePolicy(policy)
	assert.Len(t, ap.Applications, 1)
	assert.Equal(t, "AVAILABLE", ap.Applications[0].InstallType)
}

func TestApplyAppPolicy_NotSlice(t *testing.T) {
	policy := &models.Policy{
		PolicyType: models.PolicyTypeApp,
		PolicyConfig: models.JSONB{
			"applications": "not-a-slice",
		},
	}
	ap := TranslatePolicy(policy)
	assert.Nil(t, ap.Applications)
}

func TestGenerateQRCode_WiFiSSIDNoPassword(t *testing.T) {
	qr, err := GenerateQRCode("token", "https://example.com/download", "OpenWiFi", "")
	assert.NoError(t, err)
	assert.NotNil(t, qr)
	assert.Greater(t, len(qr), 0)
}
