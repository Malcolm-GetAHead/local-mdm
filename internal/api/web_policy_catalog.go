package api

// policySetting defines a single configurable policy setting.
type policySetting struct {
	Key         string // JSON key in policy_config
	Label       string // Human-readable label
	Description string
	Type        string // "bool", "int", "string", "select"
	Default     interface{}
	Options     []string // For "select" type
	Category    string   // Grouping header
	Platforms   []string // Which platforms support this ("all" = all)
}

// policySettingsCatalog is the authoritative list of valid policy settings.
var policySettingsCatalog = []policySetting{
	// Security
	{Key: "require_encryption", Label: "Require Encryption", Description: "Require full-disk encryption (FileVault/BitLocker)", Type: "bool", Default: false, Category: "Security", Platforms: []string{"all"}},
	{Key: "min_password_length", Label: "Minimum Password Length", Description: "Minimum number of characters for device passcode", Type: "int", Default: 0, Category: "Security", Platforms: []string{"all"}},
	{Key: "require_firewall", Label: "Require Firewall", Description: "Require the system firewall to be enabled", Type: "bool", Default: false, Category: "Security", Platforms: []string{"macos", "windows"}},
	{Key: "require_bitlocker", Label: "Require BitLocker", Description: "Enforce BitLocker drive encryption", Type: "bool", Default: false, Category: "Security", Platforms: []string{"windows"}},
	{Key: "encryption_method", Label: "Encryption Method", Description: "BitLocker encryption algorithm", Type: "select", Default: "XTS-AES-256", Options: []string{"XTS-AES-128", "XTS-AES-256"}, Category: "Security", Platforms: []string{"windows"}},
	{Key: "auto_lock_minutes", Label: "Auto-Lock (minutes)", Description: "Lock screen after inactivity", Type: "int", Default: 0, Category: "Security", Platforms: []string{"all"}},

	// Restrictions
	{Key: "disable_camera", Label: "Disable Camera", Description: "Prevent use of the device camera", Type: "bool", Default: false, Category: "Restrictions", Platforms: []string{"all"}},
	{Key: "disable_airdrop", Label: "Disable AirDrop", Description: "Prevent AirDrop file sharing", Type: "bool", Default: false, Category: "Restrictions", Platforms: []string{"macos"}},
	{Key: "disable_screen_capture", Label: "Disable Screen Capture", Description: "Prevent screenshots and screen recording", Type: "bool", Default: false, Category: "Restrictions", Platforms: []string{"macos", "android"}},
	{Key: "require_passcode", Label: "Require Passcode", Description: "Require a device passcode to be set", Type: "bool", Default: false, Category: "Restrictions", Platforms: []string{"all"}},

	// WiFi
	{Key: "ssid", Label: "Network Name (SSID)", Description: "WiFi network name to auto-configure", Type: "string", Default: "", Category: "WiFi", Platforms: []string{"all"}},
	{Key: "wifi_security", Label: "Security Type", Description: "WiFi authentication method", Type: "select", Default: "WPA2-Enterprise", Options: []string{"WPA2-Personal", "WPA2-Enterprise", "WPA3-Personal", "WPA3-Enterprise"}, Category: "WiFi", Platforms: []string{"all"}},
	{Key: "auto_join", Label: "Auto-Join", Description: "Automatically connect to this network", Type: "bool", Default: true, Category: "WiFi", Platforms: []string{"all"}},

	// VPN
	{Key: "vpn_server", Label: "VPN Server", Description: "VPN server hostname or IP", Type: "string", Default: "", Category: "VPN", Platforms: []string{"all"}},
	{Key: "vpn_protocol", Label: "VPN Protocol", Description: "VPN connection protocol", Type: "select", Default: "IKEv2", Options: []string{"IKEv2", "L2TP", "IPSec", "OpenVPN"}, Category: "VPN", Platforms: []string{"all"}},
	{Key: "vpn_on_demand", Label: "Connect On Demand", Description: "Automatically connect VPN when needed", Type: "bool", Default: false, Category: "VPN", Platforms: []string{"all"}},
}

// validPolicyKeys returns the set of valid keys for validation.
func validPolicyKeys() map[string]bool {
	m := make(map[string]bool, len(policySettingsCatalog))
	for _, s := range policySettingsCatalog {
		m[s.Key] = true
	}
	return m
}

// settingsByCategory groups settings by category, filtered by platform.
func settingsByCategory(platform string) []settingGroup {
	groups := map[string]*settingGroup{}
	var order []string
	for _, s := range policySettingsCatalog {
		if !settingMatchesPlatform(s, platform) {
			continue
		}
		g, ok := groups[s.Category]
		if !ok {
			g = &settingGroup{Category: s.Category}
			groups[s.Category] = g
			order = append(order, s.Category)
		}
		g.Settings = append(g.Settings, s)
	}
	var result []settingGroup
	for _, cat := range order {
		result = append(result, *groups[cat])
	}
	return result
}

func settingMatchesPlatform(s policySetting, platform string) bool {
	for _, p := range s.Platforms {
		if p == "all" || p == platform || platform == "" || platform == "all" {
			return true
		}
	}
	return false
}

type settingGroup struct {
	Category string
	Settings []policySetting
}
