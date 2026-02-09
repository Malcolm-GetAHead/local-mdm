package android

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"

	"github.com/skip2/go-qrcode"
)

// EnrollmentToken represents an enrollment token with QR code
type EnrollmentToken struct {
	Token      string `json:"token"`
	QRCodeData string `json:"qr_code_data"`
	ExpiresAt  string `json:"expires_at"`
}

// GenerateQRCode generates a QR code image for enrollment
func GenerateQRCode(token, downloadURL, wifiSSID, wifiPassword string) ([]byte, error) {
	// Create enrollment data structure
	enrollmentData := map[string]interface{}{
		"android.app.extra.PROVISIONING_DEVICE_ADMIN_COMPONENT_NAME": "com.google.android.apps.work.clouddpc/.receivers.CloudDeviceAdminReceiver",
		"android.app.extra.PROVISIONING_DEVICE_ADMIN_SIGNATURE_CHECKSUM": "I5YvS0O5hXY46mb01BlRjq4oJJGs2kuUcHvVkAPEXlg",
		"android.app.extra.PROVISIONING_DEVICE_ADMIN_PACKAGE_DOWNLOAD_LOCATION": downloadURL,
		"android.app.extra.PROVISIONING_ADMIN_EXTRAS_BUNDLE": map[string]string{
			"com.google.android.apps.work.clouddpc.EXTRA_ENROLLMENT_TOKEN": token,
		},
	}

	// Add WiFi configuration if provided
	if wifiSSID != "" {
		enrollmentData["android.app.extra.PROVISIONING_WIFI_SSID"] = wifiSSID
		if wifiPassword != "" {
			enrollmentData["android.app.extra.PROVISIONING_WIFI_PASSWORD"] = wifiPassword
		}
	}

	// Convert to JSON
	jsonData, err := json.Marshal(enrollmentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal enrollment data: %w", err)
	}

	// Generate QR code
	qr, err := qrcode.New(string(jsonData), qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, qr.Image(256)); err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateSimpleQRCode generates a simple QR code with just the token
func GenerateSimpleQRCode(token string) ([]byte, error) {
	qr, err := qrcode.Encode(token, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	return qr, nil
}
