package windows

import (
	"archive/zip"
	"bytes"
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

// PPKGConfig holds settings for generating a provisioning package.
type PPKGConfig struct {
	Name         string
	Version      string
	ServerURL    string
	DiscoveryURL string
	WiFi         *PPKGWiFiConfig
	VPN          *PPKGVPNConfig
}

// PPKGWiFiConfig holds WiFi settings for a provisioning package.
type PPKGWiFiConfig struct {
	SSID         string
	SecurityType string
	Password     string
}

// PPKGVPNConfig holds VPN settings for a provisioning package.
type PPKGVPNConfig struct {
	Name       string
	Server     string
	TunnelType string
}

// PPKGTemplate describes a pre-built provisioning template.
type PPKGTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AvailableTemplates returns the list of pre-built provisioning templates.
func AvailableTemplates() []PPKGTemplate {
	return []PPKGTemplate{
		{ID: "enrollment_only", Name: "Basic Enrollment", Description: "MDM enrollment configuration only"},
		{ID: "enrollment_wifi", Name: "Enrollment + WiFi", Description: "MDM enrollment with WiFi profile"},
		{ID: "enrollment_wifi_vpn", Name: "Enrollment + WiFi + VPN", Description: "MDM enrollment with WiFi and VPN profiles"},
	}
}

// GeneratePPKG creates a .ppkg ZIP file containing ICD XML provisioning data.
// If signer is non-nil, the package is signed and the signature + cert are included.
func GeneratePPKG(cfg PPKGConfig, signer *PPKGSigner) ([]byte, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server URL is required")
	}
	if cfg.Name == "" {
		cfg.Name = "LocalMDM Enrollment"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0"
	}
	if cfg.DiscoveryURL == "" {
		cfg.DiscoveryURL = cfg.ServerURL + "/EnrollmentServer/Discovery.svc"
	}

	xmlData, err := renderICDXML(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to render ICD XML: %w", err)
	}

	return assembleZIP(cfg.Name, xmlData, signer)
}

func renderICDXML(cfg PPKGConfig) ([]byte, error) {
	data := map[string]interface{}{
		"GUID":         uuid.New().String(),
		"Name":         cfg.Name,
		"Version":      cfg.Version,
		"DiscoveryURL": cfg.DiscoveryURL,
	}

	var wifiBlock, vpnBlock string
	if cfg.WiFi != nil && cfg.WiFi.SSID != "" {
		sec := cfg.WiFi.SecurityType
		if sec == "" {
			sec = "WPA2PSK"
		}
		wifiBlock = fmt.Sprintf(`
        <WiFi>
          <Profile>
            <SSID>%s</SSID>
            <SecurityType>%s</SecurityType>
            <Password>%s</Password>
          </Profile>
        </WiFi>`, cfg.WiFi.SSID, sec, cfg.WiFi.Password)
	}
	if cfg.VPN != nil && cfg.VPN.Server != "" {
		tt := cfg.VPN.TunnelType
		if tt == "" {
			tt = "IKEv2"
		}
		name := cfg.VPN.Name
		if name == "" {
			name = "Corporate VPN"
		}
		vpnBlock = fmt.Sprintf(`
        <VPN>
          <Profile>
            <Name>%s</Name>
            <Server>%s</Server>
            <TunnelType>%s</TunnelType>
          </Profile>
        </VPN>`, name, cfg.VPN.Server, tt)
	}
	data["WiFiBlock"] = wifiBlock
	data["VPNBlock"] = vpnBlock

	tmpl, err := template.New("icd").Parse(icdXMLTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func assembleZIP(name string, icdXML []byte, signer *PPKGSigner) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("Customizations.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to create zip entry: %w", err)
	}
	if _, err := f.Write(icdXML); err != nil {
		return nil, fmt.Errorf("failed to write zip entry: %w", err)
	}

	if signer != nil {
		sig, err := signer.Sign(icdXML)
		if err != nil {
			return nil, fmt.Errorf("failed to sign package: %w", err)
		}
		sf, err := w.Create("Signature.p7x")
		if err != nil {
			return nil, fmt.Errorf("failed to create signature entry: %w", err)
		}
		if _, err := sf.Write(sig); err != nil {
			return nil, fmt.Errorf("failed to write signature: %w", err)
		}
		cf, err := w.Create("Certificates/signing.cer")
		if err != nil {
			return nil, fmt.Errorf("failed to create cert entry: %w", err)
		}
		if _, err := cf.Write(signer.CertificatePEM()); err != nil {
			return nil, fmt.Errorf("failed to write cert: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}
	return buf.Bytes(), nil
}

const icdXMLTemplate = `<?xml version="1.0" encoding="utf-8"?>
<WindowsCustomizations>
  <PackageConfig xmlns="urn:schemas-Microsoft-com:Windows-ICD-Package-Config.v1.0">
    <ID>{{.GUID}}</ID>
    <Name>{{.Name}}</Name>
    <Version>{{.Version}}</Version>
    <OwnerType>OEM</OwnerType>
  </PackageConfig>
  <Settings xmlns="urn:schemas-microsoft-com:windows-provisioning">
    <Customizations>
      <Common>
        <Policies>
          <AllowMDMEnrollment>1</AllowMDMEnrollment>
        </Policies>
        <DMClient>
          <Provider>
            <ProviderID>LocalMDM</ProviderID>
            <DiscoveryServiceFullURL>{{.DiscoveryURL}}</DiscoveryServiceFullURL>
          </Provider>
        </DMClient>{{.WiFiBlock}}{{.VPNBlock}}
      </Common>
    </Customizations>
  </Settings>
</WindowsCustomizations>`
