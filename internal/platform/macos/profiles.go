package macos

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

// ProfileConfig holds common configuration profile fields.
type ProfileConfig struct {
	Identifier  string
	DisplayName string
	OrgName     string
	Description string
}

// WiFiProfileConfig holds WiFi configuration profile settings.
type WiFiProfileConfig struct {
	ProfileConfig
	SSID         string
	SecurityType string // WPA2, WPA3, WEP, None
	Password     string
	AutoJoin     bool
	IsHidden     bool
	ProxyType    string // None, Manual, Auto
}

// VPNProfileConfig holds VPN configuration profile settings.
type VPNProfileConfig struct {
	ProfileConfig
	VPNType       string // IKEv2, IPSec
	ServerAddress string
	RemoteID      string
	LocalID       string
	Username      string
	SharedSecret  string
}

// CertificateProfileConfig holds certificate payload settings.
type CertificateProfileConfig struct {
	ProfileConfig
	CertData    []byte
	CertFormat  string // PKCS12, PEM
	Password    string
}

// RestrictionsProfileConfig holds device restriction settings.
type RestrictionsProfileConfig struct {
	ProfileConfig
	AllowCamera          *bool
	AllowScreenCapture   *bool
	AllowAppInstallation *bool
	AllowUSBRestricted   *bool
	ForceEncryption      *bool
}

// GenerateWiFiProfile generates a macOS WiFi configuration profile.
func GenerateWiFiProfile(cfg WiFiProfileConfig) ([]byte, error) {
	if cfg.SSID == "" {
		return nil, fmt.Errorf("SSID is required")
	}
	if cfg.Identifier == "" {
		cfg.Identifier = fmt.Sprintf("com.localmdm.wifi.%s", cfg.SSID)
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = fmt.Sprintf("WiFi - %s", cfg.SSID)
	}

	encType := "WPA2"
	switch cfg.SecurityType {
	case "WPA3":
		encType = "WPA3"
	case "WEP":
		encType = "WEP"
	case "None":
		encType = "None"
	}

	data := map[string]interface{}{
		"PayloadUUID":    uuid.New().String(),
		"PayloadSubUUID": uuid.New().String(),
		"Identifier":     cfg.Identifier,
		"DisplayName":    cfg.DisplayName,
		"OrgName":        cfg.OrgName,
		"Description":    cfg.Description,
		"SSID":           cfg.SSID,
		"EncType":        encType,
		"Password":       cfg.Password,
		"AutoJoin":       cfg.AutoJoin,
		"IsHidden":       cfg.IsHidden,
	}

	return renderTemplate(wifiProfileTemplate, data)
}

// GenerateVPNProfile generates a macOS VPN configuration profile.
func GenerateVPNProfile(cfg VPNProfileConfig) ([]byte, error) {
	if cfg.ServerAddress == "" {
		return nil, fmt.Errorf("server address is required")
	}
	if cfg.Identifier == "" {
		cfg.Identifier = "com.localmdm.vpn"
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = "VPN"
	}
	if cfg.VPNType == "" {
		cfg.VPNType = "IKEv2"
	}

	data := map[string]interface{}{
		"PayloadUUID":    uuid.New().String(),
		"PayloadSubUUID": uuid.New().String(),
		"Identifier":     cfg.Identifier,
		"DisplayName":    cfg.DisplayName,
		"OrgName":        cfg.OrgName,
		"Description":    cfg.Description,
		"VPNType":        cfg.VPNType,
		"ServerAddress":  cfg.ServerAddress,
		"RemoteID":       cfg.RemoteID,
		"LocalID":        cfg.LocalID,
		"Username":       cfg.Username,
		"SharedSecret":   cfg.SharedSecret,
	}

	return renderTemplate(vpnProfileTemplate, data)
}

// GenerateCertificateProfile generates a certificate configuration profile.
func GenerateCertificateProfile(cfg CertificateProfileConfig) ([]byte, error) {
	if len(cfg.CertData) == 0 {
		return nil, fmt.Errorf("certificate data is required")
	}
	if cfg.Identifier == "" {
		cfg.Identifier = "com.localmdm.cert"
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = "Certificate"
	}
	if cfg.CertFormat == "" {
		cfg.CertFormat = "PKCS12"
	}

	payloadType := "com.apple.security.pkcs12"
	if cfg.CertFormat == "PEM" {
		payloadType = "com.apple.security.pem"
	}

	data := map[string]interface{}{
		"PayloadUUID":    uuid.New().String(),
		"PayloadSubUUID": uuid.New().String(),
		"Identifier":     cfg.Identifier,
		"DisplayName":    cfg.DisplayName,
		"OrgName":        cfg.OrgName,
		"Description":    cfg.Description,
		"PayloadType":    payloadType,
		"CertData":       base64Encode(cfg.CertData),
		"Password":       cfg.Password,
	}

	return renderTemplate(certProfileTemplate, data)
}

// GenerateRestrictionsProfile generates a device restrictions configuration profile.
func GenerateRestrictionsProfile(cfg RestrictionsProfileConfig) ([]byte, error) {
	if cfg.Identifier == "" {
		cfg.Identifier = "com.localmdm.restrictions"
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = "Restrictions"
	}

	data := map[string]interface{}{
		"PayloadUUID":    uuid.New().String(),
		"PayloadSubUUID": uuid.New().String(),
		"Identifier":     cfg.Identifier,
		"DisplayName":    cfg.DisplayName,
		"OrgName":        cfg.OrgName,
		"Description":    cfg.Description,
	}

	// Build restriction entries
	var restrictions []string
	if cfg.AllowCamera != nil {
		restrictions = append(restrictions, boolEntry("allowCamera", *cfg.AllowCamera))
	}
	if cfg.AllowScreenCapture != nil {
		restrictions = append(restrictions, boolEntry("allowScreenCapture", *cfg.AllowScreenCapture))
	}
	if cfg.AllowAppInstallation != nil {
		restrictions = append(restrictions, boolEntry("allowAppInstallation", *cfg.AllowAppInstallation))
	}
	if cfg.AllowUSBRestricted != nil {
		restrictions = append(restrictions, boolEntry("allowUSBRestrictedMode", *cfg.AllowUSBRestricted))
	}
	if cfg.ForceEncryption != nil {
		restrictions = append(restrictions, boolEntry("forceEncryptedBackup", *cfg.ForceEncryption))
	}
	data["Restrictions"] = restrictions

	return renderTemplate(restrictionsProfileTemplate, data)
}

func boolEntry(key string, val bool) string {
	v := "false"
	if val {
		v = "true"
	}
	return fmt.Sprintf("\t\t<key>%s</key>\n\t\t<%s/>", key, v)
}

func renderTemplate(tmplStr string, data interface{}) ([]byte, error) {
	tmpl, err := template.New("profile").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render profile: %w", err)
	}
	return buf.Bytes(), nil
}

const wifiProfileTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>AutoJoin</key>
			<{{if .AutoJoin}}true{{else}}false{{end}}/>
			<key>EncryptionType</key>
			<string>{{.EncType}}</string>
			<key>HIDDEN_NETWORK</key>
			<{{if .IsHidden}}true{{else}}false{{end}}/>
			<key>SSID_STR</key>
			<string>{{.SSID}}</string>{{if .Password}}
			<key>Password</key>
			<string>{{.Password}}</string>{{end}}
			<key>PayloadIdentifier</key>
			<string>{{.Identifier}}.wifi</string>
			<key>PayloadType</key>
			<string>com.apple.wifi.managed</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadSubUUID}}</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>{{.DisplayName}}</string>
	<key>PayloadIdentifier</key>
	<string>{{.Identifier}}</string>{{if .OrgName}}
	<key>PayloadOrganization</key>
	<string>{{.OrgName}}</string>{{end}}{{if .Description}}
	<key>PayloadDescription</key>
	<string>{{.Description}}</string>{{end}}
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

const vpnProfileTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>VPNType</key>
			<string>{{.VPNType}}</string>
			<key>VPN</key>
			<dict>
				<key>RemoteAddress</key>
				<string>{{.ServerAddress}}</string>
				<key>AuthenticationMethod</key>
				<string>SharedSecret</string>{{if .RemoteID}}
				<key>RemoteIdentifier</key>
				<string>{{.RemoteID}}</string>{{end}}{{if .LocalID}}
				<key>LocalIdentifier</key>
				<string>{{.LocalID}}</string>{{end}}{{if .SharedSecret}}
				<key>SharedSecret</key>
				<string>{{.SharedSecret}}</string>{{end}}{{if .Username}}
				<key>AuthName</key>
				<string>{{.Username}}</string>{{end}}
			</dict>
			<key>PayloadIdentifier</key>
			<string>{{.Identifier}}.vpn</string>
			<key>PayloadType</key>
			<string>com.apple.vpn.managed</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadSubUUID}}</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>{{.DisplayName}}</string>
	<key>PayloadIdentifier</key>
	<string>{{.Identifier}}</string>{{if .OrgName}}
	<key>PayloadOrganization</key>
	<string>{{.OrgName}}</string>{{end}}{{if .Description}}
	<key>PayloadDescription</key>
	<string>{{.Description}}</string>{{end}}
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

const certProfileTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadContent</key>
			<data>{{.CertData}}</data>{{if .Password}}
			<key>Password</key>
			<string>{{.Password}}</string>{{end}}
			<key>PayloadIdentifier</key>
			<string>{{.Identifier}}.cert</string>
			<key>PayloadType</key>
			<string>{{.PayloadType}}</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadSubUUID}}</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>{{.DisplayName}}</string>
	<key>PayloadIdentifier</key>
	<string>{{.Identifier}}</string>{{if .OrgName}}
	<key>PayloadOrganization</key>
	<string>{{.OrgName}}</string>{{end}}{{if .Description}}
	<key>PayloadDescription</key>
	<string>{{.Description}}</string>{{end}}
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

const restrictionsProfileTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>{{range .Restrictions}}
{{.}}{{end}}
			<key>PayloadIdentifier</key>
			<string>{{.Identifier}}.restrictions</string>
			<key>PayloadType</key>
			<string>com.apple.applicationaccess</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadSubUUID}}</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>{{.DisplayName}}</string>
	<key>PayloadIdentifier</key>
	<string>{{.Identifier}}</string>{{if .OrgName}}
	<key>PayloadOrganization</key>
	<string>{{.OrgName}}</string>{{end}}{{if .Description}}
	<key>PayloadDescription</key>
	<string>{{.Description}}</string>{{end}}
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`
