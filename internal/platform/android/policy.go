package android

import (
	"encoding/json"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"google.golang.org/api/androidmanagement/v1"
)

// TranslatePolicy converts a unified policy model into an Android Management API Policy.
func TranslatePolicy(policy *models.Policy) *androidmanagement.Policy {
	ap := &androidmanagement.Policy{}

	if policy.PolicyConfig == nil {
		return ap
	}

	switch policy.PolicyType {
	case models.PolicyTypeSecurity:
		applySecurityPolicy(ap, policy.PolicyConfig)
	case models.PolicyTypeRestriction:
		applyRestrictionPolicy(ap, policy.PolicyConfig)
	case models.PolicyTypeWiFi:
		applyWiFiPolicy(ap, policy.PolicyConfig)
	case models.PolicyTypeApp:
		applyAppPolicy(ap, policy.PolicyConfig)
	}

	return ap
}

func applySecurityPolicy(ap *androidmanagement.Policy, cfg models.JSONB) {
	pr := &androidmanagement.PasswordRequirements{}
	hasReqs := false

	if v, ok := cfg["min_password_length"].(float64); ok {
		pr.PasswordMinimumLength = int64(v)
		hasReqs = true
	}
	if v, ok := cfg["password_quality"].(string); ok {
		pr.PasswordQuality = v
		hasReqs = true
	}
	if v, ok := cfg["max_failed_attempts"].(float64); ok {
		pr.MaximumFailedPasswordsForWipe = int64(v)
		hasReqs = true
	}

	if hasReqs {
		ap.PasswordRequirements = pr
	}

	if v, ok := cfg["require_encryption"].(bool); ok && v {
		ap.EncryptionPolicy = "ENABLED_WITHOUT_PASSWORD"
	}

	if v, ok := cfg["lock_timeout_minutes"].(float64); ok {
		ap.MaximumTimeToLock = int64(v) * 60 * 1000
	}
}

func applyRestrictionPolicy(ap *androidmanagement.Policy, cfg models.JSONB) {
	if v, ok := cfg["allow_camera"].(bool); ok && !v {
		ap.CameraDisabled = true
	}
	if v, ok := cfg["allow_screen_capture"].(bool); ok && !v {
		ap.ScreenCaptureDisabled = true
	}
	if v, ok := cfg["allow_usb_transfer"].(bool); ok && !v {
		ap.UsbFileTransferDisabled = true
	}
	if v, ok := cfg["allow_bluetooth"].(bool); ok && !v {
		ap.BluetoothDisabled = true
	}
}

func applyWiFiPolicy(ap *androidmanagement.Policy, cfg models.JSONB) {
	ssid, _ := cfg["ssid"].(string)
	if ssid == "" {
		return
	}

	security, _ := cfg["security_type"].(string)
	if security == "" {
		security = "WPA2_PSK"
	}
	password, _ := cfg["password"].(string)

	onc := map[string]interface{}{
		"NetworkConfigurations": []map[string]interface{}{
			{
				"GUID": ssid,
				"Name": ssid,
				"Type": "WiFi",
				"WiFi": map[string]interface{}{
					"SSID":       ssid,
					"Security":   security,
					"Passphrase": password,
				},
			},
		},
	}

	oncJSON, _ := json.Marshal(onc)
	ap.OpenNetworkConfiguration = oncJSON
}

func applyAppPolicy(ap *androidmanagement.Policy, cfg models.JSONB) {
	apps, ok := cfg["applications"].([]interface{})
	if !ok {
		return
	}

	for _, a := range apps {
		appMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		packageName, _ := appMap["package_name"].(string)
		if packageName == "" {
			continue
		}

		appPolicy := &androidmanagement.ApplicationPolicy{
			PackageName:             packageName,
			InstallType:             "FORCE_INSTALLED",
			DefaultPermissionPolicy: "GRANT",
		}

		if installType, ok := appMap["install_type"].(string); ok {
			appPolicy.InstallType = installType
		}

		ap.Applications = append(ap.Applications, appPolicy)
	}
}
