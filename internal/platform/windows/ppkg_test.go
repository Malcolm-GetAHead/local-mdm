package windows

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePPKG_Basic(t *testing.T) {
	data, err := GeneratePPKG(PPKGConfig{
		ServerURL: "https://mdm.example.com",
	})
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's a valid ZIP
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, r.File, 1)
	assert.Equal(t, "Customizations.xml", r.File[0].Name)

	// Read and verify XML content
	f, err := r.File[0].Open()
	require.NoError(t, err)
	defer f.Close()
	var buf bytes.Buffer
	buf.ReadFrom(f)
	xml := buf.String()

	assert.Contains(t, xml, "LocalMDM Enrollment")
	assert.Contains(t, xml, "https://mdm.example.com/EnrollmentServer/Discovery.svc")
	assert.Contains(t, xml, "AllowMDMEnrollment")
	assert.Contains(t, xml, "LocalMDM")
	assert.NotContains(t, xml, "<WiFi>")
	assert.NotContains(t, xml, "<VPN>")
}

func TestGeneratePPKG_WithWiFi(t *testing.T) {
	data, err := GeneratePPKG(PPKGConfig{
		ServerURL: "https://mdm.example.com",
		WiFi: &PPKGWiFiConfig{
			SSID:         "CorpNet",
			SecurityType: "WPA2PSK",
			Password:     "secret123",
		},
	})
	require.NoError(t, err)

	xml := extractXMLFromPPKG(t, data)
	assert.Contains(t, xml, "CorpNet")
	assert.Contains(t, xml, "WPA2PSK")
	assert.Contains(t, xml, "secret123")
}

func TestGeneratePPKG_WithWiFiAndVPN(t *testing.T) {
	data, err := GeneratePPKG(PPKGConfig{
		Name:      "Full Config",
		ServerURL: "https://mdm.example.com",
		WiFi: &PPKGWiFiConfig{
			SSID:     "CorpNet",
			Password: "wifipass",
		},
		VPN: &PPKGVPNConfig{
			Name:       "Corp VPN",
			Server:     "vpn.example.com",
			TunnelType: "IKEv2",
		},
	})
	require.NoError(t, err)

	xml := extractXMLFromPPKG(t, data)
	assert.Contains(t, xml, "Full Config")
	assert.Contains(t, xml, "CorpNet")
	assert.Contains(t, xml, "vpn.example.com")
	assert.Contains(t, xml, "IKEv2")
	assert.Contains(t, xml, "Corp VPN")
}

func TestGeneratePPKG_CustomDiscoveryURL(t *testing.T) {
	data, err := GeneratePPKG(PPKGConfig{
		ServerURL:    "https://mdm.example.com",
		DiscoveryURL: "https://custom.example.com/discovery",
	})
	require.NoError(t, err)

	xml := extractXMLFromPPKG(t, data)
	assert.Contains(t, xml, "https://custom.example.com/discovery")
	assert.NotContains(t, xml, "EnrollmentServer")
}

func TestGeneratePPKG_MissingServerURL(t *testing.T) {
	_, err := GeneratePPKG(PPKGConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server URL is required")
}

func TestGeneratePPKG_Defaults(t *testing.T) {
	data, err := GeneratePPKG(PPKGConfig{
		ServerURL: "https://mdm.test.com",
		WiFi:      &PPKGWiFiConfig{SSID: "Net"},
		VPN:       &PPKGVPNConfig{Server: "vpn.test.com"},
	})
	require.NoError(t, err)

	xml := extractXMLFromPPKG(t, data)
	assert.Contains(t, xml, "WPA2PSK")       // default WiFi security
	assert.Contains(t, xml, "IKEv2")         // default VPN tunnel
	assert.Contains(t, xml, "Corporate VPN") // default VPN name
}

func TestAvailableTemplates(t *testing.T) {
	templates := AvailableTemplates()
	assert.Len(t, templates, 3)
	assert.Equal(t, "enrollment_only", templates[0].ID)
	assert.Equal(t, "enrollment_wifi", templates[1].ID)
	assert.Equal(t, "enrollment_wifi_vpn", templates[2].ID)
}

func extractXMLFromPPKG(t *testing.T, data []byte) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.Len(t, r.File, 1)
	f, err := r.File[0].Open()
	require.NoError(t, err)
	defer f.Close()
	var buf bytes.Buffer
	buf.ReadFrom(f)
	return buf.String()
}
