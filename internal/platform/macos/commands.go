package macos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// CommandSender sends MDM commands to macOS devices via NanoMDM API.
type CommandSender struct {
	nanomdmURL string
	apiKey     string
	client     *http.Client
}

// NewCommandSender creates a CommandSender targeting a NanoMDM server.
func NewCommandSender(nanomdmURL, apiKey string) *CommandSender {
	return &CommandSender{
		nanomdmURL: nanomdmURL,
		apiKey:     apiKey,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// EnqueueRequest is the NanoMDM enqueue API request body.
type EnqueueRequest struct {
	UDIDs      []string `json:"udids"`
	CommandRaw []byte   `json:"command_raw,omitempty"`
	RequestType string  `json:"request_type,omitempty"`
}

// EnqueueResponse is the NanoMDM enqueue API response.
type EnqueueResponse struct {
	CommandUUID string            `json:"command_uuid"`
	Status      map[string]string `json:"status"`
}

// SendCommand enqueues a raw plist command to a device via NanoMDM.
func (c *CommandSender) SendCommand(ctx context.Context, udid string, commandPlist []byte) (*EnqueueResponse, error) {
	reqBody := EnqueueRequest{
		UDIDs:      []string{udid},
		CommandRaw: commandPlist,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal enqueue request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/enqueue", c.nanomdmURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send command to NanoMDM: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NanoMDM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result EnqueueResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode NanoMDM response: %w", err)
	}
	return &result, nil
}

// BuildDeviceInformationCommand builds a DeviceInformation MDM command plist.
func BuildDeviceInformationCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>DeviceInformation</string>
		<key>Queries</key>
		<array>
			<string>DeviceName</string>
			<string>OSVersion</string>
			<string>BuildVersion</string>
			<string>ModelName</string>
			<string>Model</string>
			<string>ProductName</string>
			<string>SerialNumber</string>
			<string>UDID</string>
			<string>WiFiMAC</string>
			<string>BatteryLevel</string>
			<string>AvailableDeviceCapacity</string>
			<string>DeviceCapacity</string>
		</array>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildSecurityInfoCommand builds a SecurityInfo MDM command plist.
func BuildSecurityInfoCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>SecurityInfo</string>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildProfileListCommand builds a ProfileList MDM command plist.
func BuildProfileListCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>ProfileList</string>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildInstalledApplicationListCommand builds an InstalledApplicationList command.
func BuildInstalledApplicationListCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>InstalledApplicationList</string>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildCertificateListCommand builds a CertificateList MDM command plist.
func BuildCertificateListCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>CertificateList</string>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildInstallProfileCommand builds an InstallProfile MDM command with the given mobileconfig payload.
func BuildInstallProfileCommand(profileData []byte) ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>InstallProfile</string>
		<key>Payload</key>
		<data>%s</data>
	</dict>
</dict>
</plist>`, cmdUUID, base64Encode(profileData))
	return []byte(plist), cmdUUID
}

// BuildRemoveProfileCommand builds a RemoveProfile MDM command.
func BuildRemoveProfileCommand(profileIdentifier string) ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>RemoveProfile</string>
		<key>Identifier</key>
		<string>%s</string>
	</dict>
</dict>
</plist>`, cmdUUID, profileIdentifier)
	return []byte(plist), cmdUUID
}

// BuildDeviceLockCommand builds a DeviceLock MDM command.
func BuildDeviceLockCommand(pin string, message string) ([]byte, string) {
	cmdUUID := uuid.New().String()
	pinEntry := ""
	if pin != "" {
		pinEntry = fmt.Sprintf(`		<key>PIN</key>
		<string>%s</string>`, pin)
	}
	msgEntry := ""
	if message != "" {
		msgEntry = fmt.Sprintf(`		<key>Message</key>
		<string>%s</string>`, message)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>DeviceLock</string>
%s
%s
	</dict>
</dict>
</plist>`, cmdUUID, pinEntry, msgEntry)
	return []byte(plist), cmdUUID
}

// BuildEraseDeviceCommand builds an EraseDevice MDM command.
func BuildEraseDeviceCommand(pin string) ([]byte, string) {
	cmdUUID := uuid.New().String()
	pinEntry := ""
	if pin != "" {
		pinEntry = fmt.Sprintf(`		<key>PIN</key>
		<string>%s</string>`, pin)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>EraseDevice</string>
%s
	</dict>
</dict>
</plist>`, cmdUUID, pinEntry)
	return []byte(plist), cmdUUID
}

// BuildRestartDeviceCommand builds a RestartDevice MDM command.
func BuildRestartDeviceCommand() ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>RestartDevice</string>
	</dict>
</dict>
</plist>`, cmdUUID)
	return []byte(plist), cmdUUID
}

// BuildInstallApplicationCommand builds an InstallApplication MDM command.
func BuildInstallApplicationCommand(itunesStoreID int, manifestURL string) ([]byte, string) {
	cmdUUID := uuid.New().String()
	var sourceEntry string
	if itunesStoreID > 0 {
		sourceEntry = fmt.Sprintf(`		<key>iTunesStoreID</key>
		<integer>%d</integer>`, itunesStoreID)
	} else if manifestURL != "" {
		sourceEntry = fmt.Sprintf(`		<key>ManifestURL</key>
		<string>%s</string>`, manifestURL)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>InstallApplication</string>
%s
	</dict>
</dict>
</plist>`, cmdUUID, sourceEntry)
	return []byte(plist), cmdUUID
}

// BuildRemoveApplicationCommand builds a RemoveApplication MDM command.
func BuildRemoveApplicationCommand(bundleID string) ([]byte, string) {
	cmdUUID := uuid.New().String()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>%s</string>
	<key>Command</key>
	<dict>
		<key>RequestType</key>
		<string>RemoveApplication</string>
		<key>Identifier</key>
		<string>%s</string>
	</dict>
</dict>
</plist>`, cmdUUID, bundleID)
	return []byte(plist), cmdUUID
}
