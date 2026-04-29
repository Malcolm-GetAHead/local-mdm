package api

import (
	"encoding/pem"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- settingsByCategory ---

func TestSettingsByCategory_Windows(t *testing.T) {
	groups := settingsByCategory("windows")
	cats := make(map[string]bool)
	for _, g := range groups {
		cats[g.Category] = true
	}
	assert.True(t, cats["Security"], "should include Security")
	assert.True(t, cats["Restrictions"], "should include Restrictions")
}

func TestSettingsByCategory_MacOS(t *testing.T) {
	groups := settingsByCategory("macos")
	found := false
	for _, g := range groups {
		for _, s := range g.Settings {
			if s.Key == "disable_airdrop" {
				found = true
			}
		}
	}
	assert.True(t, found, "macos should include AirDrop setting")
}

func TestSettingsByCategory_All(t *testing.T) {
	groups := settingsByCategory("all")
	for _, g := range groups {
		for _, s := range g.Settings {
			assert.Contains(t, s.Platforms, "all", "platform=all should only return cross-platform settings, got %s with %v", s.Key, s.Platforms)
		}
	}
	assert.NotEmpty(t, groups)
}

func TestSettingsByCategory_Empty(t *testing.T) {
	// Empty string behaves like "all" — only cross-platform settings
	groups := settingsByCategory("")
	for _, g := range groups {
		for _, s := range g.Settings {
			assert.Contains(t, s.Platforms, "all")
		}
	}
}

// --- settingMatchesPlatform ---

func TestSettingMatchesPlatform(t *testing.T) {
	allSetting := policySetting{Platforms: []string{"all"}}
	winSetting := policySetting{Platforms: []string{"windows"}}
	macSetting := policySetting{Platforms: []string{"macos"}}
	multiSetting := policySetting{Platforms: []string{"macos", "windows"}}

	// platform="all" matches only Platforms=["all"]
	assert.True(t, settingMatchesPlatform(allSetting, "all"))
	assert.False(t, settingMatchesPlatform(winSetting, "all"))
	assert.False(t, settingMatchesPlatform(macSetting, "all"))

	// platform="" behaves like "all"
	assert.True(t, settingMatchesPlatform(allSetting, ""))
	assert.False(t, settingMatchesPlatform(winSetting, ""))

	// platform="windows" matches "all" and "windows"
	assert.True(t, settingMatchesPlatform(allSetting, "windows"))
	assert.True(t, settingMatchesPlatform(winSetting, "windows"))
	assert.False(t, settingMatchesPlatform(macSetting, "windows"))
	assert.True(t, settingMatchesPlatform(multiSetting, "windows"))

	// platform="macos" matches "all" and "macos"
	assert.True(t, settingMatchesPlatform(allSetting, "macos"))
	assert.True(t, settingMatchesPlatform(macSetting, "macos"))
	assert.False(t, settingMatchesPlatform(winSetting, "macos"))
	assert.True(t, settingMatchesPlatform(multiSetting, "macos"))
}

// --- decodePEMBlock ---

func TestDecodePEMBlock_Valid(t *testing.T) {
	derBytes := []byte("fake certificate data")
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	block, rest := decodePEMBlock(pemData)
	require.NotNil(t, block)
	assert.Equal(t, derBytes, block)
	assert.Empty(t, rest)
}

func TestDecodePEMBlock_Invalid(t *testing.T) {
	block, _ := decodePEMBlock([]byte("not pem data"))
	assert.Nil(t, block)
}

func TestDecodePEMBlock_Multiple(t *testing.T) {
	first := []byte("first block")
	second := []byte("second block")
	pemData := append(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: first}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: second})...,
	)

	block, rest := decodePEMBlock(pemData)
	require.NotNil(t, block)
	assert.Equal(t, first, block)

	// rest should decode to the second block
	block2, _ := decodePEMBlock(rest)
	require.NotNil(t, block2)
	assert.Equal(t, second, block2)
}

// --- buildPlatformDetails ---

func TestBuildPlatformDetails_Nil(t *testing.T) {
	assert.Nil(t, buildPlatformDetails(nil))
}

func TestBuildPlatformDetails_Empty(t *testing.T) {
	assert.Nil(t, buildPlatformDetails(models.JSONB{}))
}

func TestBuildPlatformDetails_WithData(t *testing.T) {
	pd := models.JSONB{
		"serial":             "ABC123",
		"hostname":           "test-host",
		"FileVaultEnabled":   true,
		"firewall_enabled":   false,
		"installed_profiles": []interface{}{},  // should be skipped
		"installed_apps":     []interface{}{},  // should be skipped
	}

	groups := buildPlatformDetails(pd)
	require.NotNil(t, groups)

	catMap := map[string][]platformDetailItem{}
	for _, g := range groups {
		catMap[g.Category] = g.Items
	}

	// Hardware category should have serial
	hw, ok := catMap["Hardware"]
	require.True(t, ok, "should have Hardware category")
	found := false
	for _, item := range hw {
		if item.Label == "Serial Number" {
			assert.Equal(t, "ABC123", item.Value)
			found = true
		}
	}
	assert.True(t, found, "should find Serial Number in Hardware")

	// Security category should have bool items
	sec, ok := catMap["Security"]
	require.True(t, ok, "should have Security category")
	for _, item := range sec {
		if item.Label == "FileVault" {
			assert.Equal(t, "bool", item.Type)
			assert.True(t, item.BoolVal)
		}
		if item.Label == "Firewall" {
			assert.Equal(t, "bool", item.Type)
			assert.False(t, item.BoolVal)
		}
	}

	// Verify categories are sorted per the defined order
	var catOrder []string
	for _, g := range groups {
		catOrder = append(catOrder, g.Category)
	}
	expected := []string{"Hardware", "Network", "Security"}
	assert.Equal(t, expected, catOrder)
}

func TestBuildPlatformDetails_SkipKeys(t *testing.T) {
	skipKeys := []string{
		"installed_profiles", "installed_apps", "installed_profiles_count",
		"installed_apps_count", "certificates", "certificates_count",
		"available_os_updates", "active_extensions", "local_users",
	}
	pd := models.JSONB{}
	for _, k := range skipKeys {
		pd[k] = "should be skipped"
	}
	pd["serial"] = "KEEP"

	groups := buildPlatformDetails(pd)
	require.NotNil(t, groups)
	// Only serial should remain
	total := 0
	for _, g := range groups {
		total += len(g.Items)
	}
	assert.Equal(t, 1, total)
}

func TestDecodeBitLockerStatus(t *testing.T) {
	assert.Equal(t, "Encrypted (Compliant)", decodeBitLockerStatus("0"))
	assert.Contains(t, decodeBitLockerStatus("2"), "Protection off or suspended")
	assert.Contains(t, decodeBitLockerStatus("4"), "OS volume unprotected")
	// Bitmask: 6 = bits 1+2
	result := decodeBitLockerStatus("6")
	assert.Contains(t, result, "Protection off or suspended")
	assert.Contains(t, result, "OS volume unprotected")
	// Non-numeric
	assert.Equal(t, "abc", decodeBitLockerStatus("abc"))
}

func TestBuildPlatformDetails_WindowsTranslations(t *testing.T) {
	pd := models.JSONB{
		"bitlocker_status": "2",
		"processor_arch":   "12",
		"total_ram":        "4096",
		"total_storage":    "31844",
	}
	groups := buildPlatformDetails(pd)
	items := map[string]string{}
	for _, g := range groups {
		for _, item := range g.Items {
			items[item.Label] = item.Value
		}
	}
	assert.Contains(t, items["BitLocker Status"], "Protection off or suspended")
	assert.Equal(t, "ARM64", items["Processor Architecture"])
	assert.Equal(t, "4.0 GB", items["Total RAM"])
	assert.Equal(t, "31.1 GB", items["Total Storage"])
}

// --- validPolicyKeys ---

func TestValidPolicyKeys(t *testing.T) {
	keys := validPolicyKeys()
	assert.True(t, keys["require_encryption"])
	assert.True(t, keys["ssid"])
	assert.True(t, keys["vpn_server"])
	assert.True(t, keys["disable_camera"])
	assert.True(t, keys["vpn_protocol"])
	assert.False(t, keys["nonexistent_key"])
	assert.Equal(t, len(policySettingsCatalog), len(keys))
}
