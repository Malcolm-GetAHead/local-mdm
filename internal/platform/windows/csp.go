package windows

import (
	"fmt"
	"strconv"

	"github.com/malcolm-getahead/local-mdm/internal/models"
)

// CSPCommand represents a single CSP operation to be added to a SyncML response.
type CSPCommand struct {
	Operation string // "Replace", "Get", "Exec", "Delete"
	URI       string
	Value     string
	Format    string // "int", "chr", "bool", "xml", "b64"
}

// ApplyCSPCommands adds CSP commands to a SyncML response, starting at cmdID.
// Returns the next available cmdID.
func ApplyCSPCommands(resp *SyncML, cmdID int, cmds []CSPCommand) int {
	for _, c := range cmds {
		id := strconv.Itoa(cmdID)
		switch c.Operation {
		case "Replace":
			resp.AddReplace(id, c.URI, c.Value, c.Format)
		case "Get":
			resp.AddGet(id, c.URI)
		case "Exec":
			resp.AddExec(id, c.URI)
		}
		cmdID++
	}
	return cmdID
}

// BuildPolicyCSPCommands generates SyncML commands for Windows security policy settings.
func BuildPolicyCSPCommands(data models.JSONB) []CSPCommand {
	var cmds []CSPCommand
	base := "./Vendor/MSFT/Policy/Config"

	if v, ok := intFromJSONB(data, "min_password_length"); ok {
		cmds = append(cmds, CSPCommand{"Replace", base + "/DeviceLock/MinDevicePasswordLength", strconv.Itoa(v), "int"})
	}
	if v, ok := boolFromJSONB(data, "password_complexity"); ok && v {
		cmds = append(cmds, CSPCommand{"Replace", base + "/DeviceLock/AlphanumericDevicePasswordRequired", "1", "int"})
	}
	if v, ok := intFromJSONB(data, "password_expiration_days"); ok {
		cmds = append(cmds, CSPCommand{"Replace", base + "/DeviceLock/DevicePasswordExpiration", strconv.Itoa(v), "int"})
	}
	if v, ok := intFromJSONB(data, "max_failed_attempts"); ok {
		cmds = append(cmds, CSPCommand{"Replace", base + "/DeviceLock/MaxDevicePasswordFailedAttempts", strconv.Itoa(v), "int"})
	}
	if v, ok := intFromJSONB(data, "lock_timeout_minutes"); ok {
		cmds = append(cmds, CSPCommand{"Replace", base + "/DeviceLock/MaxInactivityTimeDeviceLock", strconv.Itoa(v), "int"})
	}
	if v, ok := boolFromJSONB(data, "require_encryption"); ok && v {
		cmds = append(cmds, CSPCommand{"Replace", base + "/BitLocker/RequireDeviceEncryption", "1", "int"})
	}

	return cmds
}

// BuildWiFiCSPCommands generates SyncML commands for WiFi profile deployment.
func BuildWiFiCSPCommands(data models.JSONB) ([]CSPCommand, error) {
	ssid, _ := data["ssid"].(string)
	if ssid == "" {
		return nil, fmt.Errorf("ssid is required for WiFi CSP")
	}

	security, _ := data["security_type"].(string)
	if security == "" {
		security = "WPA2PSK"
	}
	password, _ := data["password"].(string)

	profileXML := fmt.Sprintf(`<WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1">
  <name>%s</name>
  <SSIDConfig><SSID><name>%s</name></SSID></SSIDConfig>
  <connectionType>ESS</connectionType>
  <connectionMode>auto</connectionMode>
  <MSM><security>
    <authEncryption>
      <authentication>%s</authentication>
      <encryption>AES</encryption>
      <useOneX>false</useOneX>
    </authEncryption>
    <sharedKey><keyType>passPhrase</keyType><protected>false</protected><keyMaterial>%s</keyMaterial></sharedKey>
  </security></MSM>
</WLANProfile>`, ssid, ssid, security, password)

	uri := fmt.Sprintf("./Vendor/MSFT/WiFi/Profile/%s/WlanXml", ssid)
	return []CSPCommand{
		{"Replace", uri, profileXML, "chr"},
	}, nil
}

// BuildVPNCSPCommands generates SyncML commands for VPN profile deployment.
func BuildVPNCSPCommands(data models.JSONB) ([]CSPCommand, error) {
	name, _ := data["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required for VPN CSP")
	}
	server, _ := data["server"].(string)
	if server == "" {
		return nil, fmt.Errorf("server is required for VPN CSP")
	}

	tunnelType, _ := data["tunnel_type"].(string)
	if tunnelType == "" {
		tunnelType = "IKEv2"
	}

	base := fmt.Sprintf("./Vendor/MSFT/VPNv2/%s", name)
	cmds := []CSPCommand{
		{"Replace", base + "/ServerList", server, "chr"},
		{"Replace", base + "/NativeProfile/NativeProtocolType", tunnelType, "chr"},
		{"Replace", base + "/NativeProfile/Authentication/MachineMethod", "Certificate", "chr"},
		{"Replace", base + "/RememberCredentials", "true", "bool"},
	}

	if v, ok := boolFromJSONB(data, "always_on"); ok && v {
		cmds = append(cmds, CSPCommand{"Replace", base + "/AlwaysOn", "true", "bool"})
	}

	return cmds, nil
}

// BuildDeviceLockCSPCommands generates SyncML commands for remote lock/wipe/pin reset.
func BuildDeviceLockCSPCommands(action string, data models.JSONB) []CSPCommand {
	switch action {
	case "lock":
		return []CSPCommand{
			{"Exec", "./Vendor/MSFT/RemoteLock/Lock", "", ""},
		}
	case "wipe":
		return []CSPCommand{
			{"Exec", "./Vendor/MSFT/RemoteWipe/doWipe", "", ""},
		}
	case "pin_reset":
		pin, _ := data["new_pin"].(string)
		if pin == "" {
			return nil
		}
		return []CSPCommand{
			{"Replace", "./Vendor/MSFT/RemoteLock/NewPINValue", pin, "chr"},
			{"Exec", "./Vendor/MSFT/RemoteLock/LockAndResetPIN", "", ""},
		}
	}
	return nil
}

// BuildAppInventoryCSPCommands generates SyncML Get commands for app inventory.
func BuildAppInventoryCSPCommands() []CSPCommand {
	return []CSPCommand{
		{"Get", "./Vendor/MSFT/EnterpriseModernAppManagement/AppManagement/AppStore", "", ""},
		{"Get", "./Vendor/MSFT/EnterpriseModernAppManagement/AppManagement/nonStore", "", ""},
		{"Get", "./Vendor/MSFT/EnterpriseModernAppManagement/AppManagement/System", "", ""},
	}
}

// helpers

func intFromJSONB(data models.JSONB, key string) (int, bool) {
	v, ok := data[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

func boolFromJSONB(data models.JSONB, key string) (bool, bool) {
	v, ok := data[key]
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		return b == "true" || b == "1", true
	case float64:
		return b != 0, true
	}
	return false, false
}
