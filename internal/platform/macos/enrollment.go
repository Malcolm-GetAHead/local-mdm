package macos

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"text/template"

	"github.com/google/uuid"
)

// EnrollmentProfile represents a macOS enrollment profile
type EnrollmentProfile struct {
	PayloadIdentifier      string
	PayloadUUID            string
	PayloadDisplayName     string
	PayloadDescription     string
	PayloadOrganization    string
	ServerURL              string
	CheckInURL             string
	Topic                  string
	SCEPURL                string
	SCEPChallenge          string
	ServerCertificateData  string
}

const profileTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadType</key>
			<string>com.apple.security.scep</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>PayloadIdentifier</key>
			<string>{{.PayloadIdentifier}}.scep</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadUUID}}-scep</string>
			<key>PayloadDisplayName</key>
			<string>SCEP</string>
			<key>PayloadDescription</key>
			<string>Configures SCEP</string>
			<key>PayloadOrganization</key>
			<string>{{.PayloadOrganization}}</string>
			<key>PayloadContent</key>
			<dict>
				<key>URL</key>
				<string>{{.SCEPURL}}</string>
				<key>Challenge</key>
				<string>{{.SCEPChallenge}}</string>
				<key>Keysize</key>
				<integer>2048</integer>
				<key>Key Type</key>
				<string>RSA</string>
				<key>Key Usage</key>
				<integer>5</integer>
			</dict>
		</dict>
		<dict>
			<key>PayloadType</key>
			<string>com.apple.mdm</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>PayloadIdentifier</key>
			<string>{{.PayloadIdentifier}}.mdm</string>
			<key>PayloadUUID</key>
			<string>{{.PayloadUUID}}-mdm</string>
			<key>PayloadDisplayName</key>
			<string>MDM</string>
			<key>PayloadDescription</key>
			<string>Configures MDM</string>
			<key>PayloadOrganization</key>
			<string>{{.PayloadOrganization}}</string>
			<key>IdentityCertificateUUID</key>
			<string>{{.PayloadUUID}}-scep</string>
			<key>Topic</key>
			<string>{{.Topic}}</string>
			<key>ServerURL</key>
			<string>{{.ServerURL}}</string>
			<key>CheckInURL</key>
			<string>{{.CheckInURL}}</string>
			<key>CheckOutWhenRemoved</key>
			<true/>
			<key>SignMessage</key>
			<true/>
			<key>AccessRights</key>
			<integer>8191</integer>
			<key>ServerCapabilities</key>
			<array>
				<string>com.apple.mdm.per-user-connections</string>
			</array>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>{{.PayloadDisplayName}}</string>
	<key>PayloadIdentifier</key>
	<string>{{.PayloadIdentifier}}</string>
	<key>PayloadOrganization</key>
	<string>{{.PayloadOrganization}}</string>
	<key>PayloadRemovalDisallowed</key>
	<false/>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

// GenerateEnrollmentProfile generates a .mobileconfig enrollment profile
func GenerateEnrollmentProfile(enterpriseID uuid.UUID, serverURL, scepURL, topic, challenge, orgName string, caCert *x509.Certificate) ([]byte, error) {
	payloadUUID := uuid.New().String()
	
	var caCertPEM string
	if caCert != nil {
		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCert.Raw,
		})
		caCertPEM = string(certPEM)
	}

	profile := EnrollmentProfile{
		PayloadIdentifier:     fmt.Sprintf("com.localmdm.%s", enterpriseID.String()),
		PayloadUUID:           payloadUUID,
		PayloadDisplayName:    "MDM Enrollment",
		PayloadDescription:    "Enrolls device into MDM",
		PayloadOrganization:   orgName,
		ServerURL:             serverURL + "/mdm",
		CheckInURL:            serverURL + "/checkin",
		Topic:                 topic,
		SCEPURL:               scepURL,
		SCEPChallenge:         challenge,
		ServerCertificateData: caCertPEM,
	}

	tmpl, err := template.New("profile").Parse(profileTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, profile); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}
