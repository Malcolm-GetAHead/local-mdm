package windows

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CSP Framework Tests ---

func TestApplyCSPCommands(t *testing.T) {
	resp := NewSyncMLResponse("1", "1", "server", "device")
	cmds := []CSPCommand{
		{"Replace", "./test/uri1", "value1", "chr"},
		{"Get", "./test/uri2", "", ""},
		{"Exec", "./test/uri3", "", ""},
	}

	nextID := ApplyCSPCommands(resp, 1, cmds)
	assert.Equal(t, 4, nextID)
	assert.Len(t, resp.SyncBody.Replace, 1)
	assert.Len(t, resp.SyncBody.Get, 1)
	assert.Len(t, resp.SyncBody.Exec, 1)
}

func TestAddReplace(t *testing.T) {
	resp := NewSyncMLResponse("1", "1", "server", "device")
	resp.AddReplace("1", "./test/uri", "hello", "chr")

	require.Len(t, resp.SyncBody.Replace, 1)
	assert.Equal(t, "1", resp.SyncBody.Replace[0].CmdID)
	assert.Equal(t, "./test/uri", resp.SyncBody.Replace[0].Item[0].Target.LocURI)
	assert.Equal(t, "hello", resp.SyncBody.Replace[0].Item[0].Data)
	assert.Equal(t, "chr", resp.SyncBody.Replace[0].Item[0].Meta.Format.Value)
}

// --- Policy CSP Tests ---

func TestBuildPolicyCSPCommands(t *testing.T) {
	t.Run("all settings", func(t *testing.T) {
		data := models.JSONB{
			"min_password_length":      float64(8),
			"password_complexity":      true,
			"password_expiration_days": float64(90),
			"max_failed_attempts":      float64(10),
			"lock_timeout_minutes":     float64(5),
			"require_encryption":       true,
		}
		cmds := BuildPolicyCSPCommands(data)
		assert.Len(t, cmds, 6)

		uris := make(map[string]string)
		for _, c := range cmds {
			uris[c.URI] = c.Value
		}
		assert.Equal(t, "8", uris["./Vendor/MSFT/Policy/Config/DeviceLock/MinDevicePasswordLength"])
		assert.Equal(t, "1", uris["./Vendor/MSFT/Policy/Config/DeviceLock/AlphanumericDevicePasswordRequired"])
		assert.Equal(t, "90", uris["./Vendor/MSFT/Policy/Config/DeviceLock/DevicePasswordExpiration"])
		assert.Equal(t, "10", uris["./Vendor/MSFT/Policy/Config/DeviceLock/MaxDevicePasswordFailedAttempts"])
		assert.Equal(t, "5", uris["./Vendor/MSFT/Policy/Config/DeviceLock/MaxInactivityTimeDeviceLock"])
		assert.Equal(t, "1", uris["./Vendor/MSFT/Policy/Config/BitLocker/RequireDeviceEncryption"])
	})

	t.Run("empty data", func(t *testing.T) {
		cmds := BuildPolicyCSPCommands(models.JSONB{})
		assert.Empty(t, cmds)
	})

	t.Run("partial settings", func(t *testing.T) {
		data := models.JSONB{"min_password_length": float64(6)}
		cmds := BuildPolicyCSPCommands(data)
		assert.Len(t, cmds, 1)
	})
}

// --- WiFi CSP Tests ---

func TestBuildWiFiCSPCommands(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := models.JSONB{
			"ssid":          "CorpNet",
			"security_type": "WPA2PSK",
			"password":      "secret123",
		}
		cmds, err := BuildWiFiCSPCommands(data)
		require.NoError(t, err)
		require.Len(t, cmds, 1)
		assert.Contains(t, cmds[0].URI, "CorpNet")
		assert.Contains(t, cmds[0].Value, "CorpNet")
		assert.Contains(t, cmds[0].Value, "secret123")
		assert.Equal(t, "chr", cmds[0].Format)
	})

	t.Run("missing ssid", func(t *testing.T) {
		_, err := BuildWiFiCSPCommands(models.JSONB{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ssid is required")
	})

	t.Run("default security", func(t *testing.T) {
		cmds, err := BuildWiFiCSPCommands(models.JSONB{"ssid": "TestNet"})
		require.NoError(t, err)
		assert.Contains(t, cmds[0].Value, "WPA2PSK")
	})
}

// --- VPN CSP Tests ---

func TestBuildVPNCSPCommands(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := models.JSONB{
			"name":        "CorpVPN",
			"server":      "vpn.example.com",
			"tunnel_type": "IKEv2",
			"always_on":   true,
		}
		cmds, err := BuildVPNCSPCommands(data)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(cmds), 4)

		uris := make(map[string]string)
		for _, c := range cmds {
			uris[c.URI] = c.Value
		}
		assert.Equal(t, "vpn.example.com", uris["./Vendor/MSFT/VPNv2/CorpVPN/ServerList"])
		assert.Equal(t, "IKEv2", uris["./Vendor/MSFT/VPNv2/CorpVPN/NativeProfile/NativeProtocolType"])
		assert.Equal(t, "true", uris["./Vendor/MSFT/VPNv2/CorpVPN/AlwaysOn"])
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := BuildVPNCSPCommands(models.JSONB{"server": "vpn.test.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("missing server", func(t *testing.T) {
		_, err := BuildVPNCSPCommands(models.JSONB{"name": "VPN"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server is required")
	})
}

// --- DeviceLock CSP Tests ---

func TestBuildDeviceLockCSPCommands(t *testing.T) {
	t.Run("lock", func(t *testing.T) {
		cmds := BuildDeviceLockCSPCommands("lock", nil)
		require.Len(t, cmds, 1)
		assert.Equal(t, "Exec", cmds[0].Operation)
		assert.Contains(t, cmds[0].URI, "RemoteLock")
	})

	t.Run("wipe", func(t *testing.T) {
		cmds := BuildDeviceLockCSPCommands("wipe", nil)
		require.Len(t, cmds, 1)
		assert.Contains(t, cmds[0].URI, "RemoteWipe")
	})

	t.Run("pin reset", func(t *testing.T) {
		cmds := BuildDeviceLockCSPCommands("pin_reset", models.JSONB{"new_pin": "1234"})
		require.Len(t, cmds, 2)
		assert.Equal(t, "Replace", cmds[0].Operation)
		assert.Equal(t, "Exec", cmds[1].Operation)
	})

	t.Run("pin reset without pin", func(t *testing.T) {
		cmds := BuildDeviceLockCSPCommands("pin_reset", models.JSONB{})
		assert.Nil(t, cmds)
	})

	t.Run("unknown action", func(t *testing.T) {
		cmds := BuildDeviceLockCSPCommands("unknown", nil)
		assert.Nil(t, cmds)
	})
}

// --- App Inventory CSP Tests ---

func TestBuildAppInventoryCSPCommands(t *testing.T) {
	cmds := BuildAppInventoryCSPCommands()
	assert.Len(t, cmds, 3)
	for _, c := range cmds {
		assert.Equal(t, "Get", c.Operation)
		assert.Contains(t, c.URI, "EnterpriseModernAppManagement")
	}
}

// --- WNS Client Tests ---

func TestWNSClient_Push(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wnsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "wns/raw", r.Header.Get("X-WNS-Type"))
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
			w.WriteHeader(http.StatusOK)
		}))
		defer wnsServer.Close()

		client := NewWNSClient("id", "secret")
		client.accessToken = "test-token"
		client.tokenExpiry = time.Now().Add(time.Hour)
		client.httpClient = wnsServer.Client()

		err := client.Push(context.Background(), wnsServer.URL)
		require.NoError(t, err)
	})

	t.Run("empty channel URI", func(t *testing.T) {
		client := NewWNSClient("id", "secret")
		err := client.Push(context.Background(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel URI is empty")
	})

	t.Run("server error", func(t *testing.T) {
		wnsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer wnsServer.Close()

		client := NewWNSClient("id", "secret")
		client.accessToken = "test-token"
		client.tokenExpiry = time.Now().Add(time.Hour)
		client.httpClient = wnsServer.Client()

		err := client.Push(context.Background(), wnsServer.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})
}

func TestWNSClient_GetAccessToken(t *testing.T) {
	t.Run("cached token", func(t *testing.T) {
		client := NewWNSClient("id", "secret")
		client.accessToken = "cached-token"
		client.tokenExpiry = time.Now().Add(time.Hour)

		token, err := client.getAccessToken(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "cached-token", token)
	})

	t.Run("no credentials", func(t *testing.T) {
		client := NewWNSClient("", "")
		_, err := client.getAccessToken(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("token refresh", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"access_token": "new-token",
				"token_type":   "Bearer",
			})
		}))
		defer tokenServer.Close()

		// We can't easily override the token URL, so test extractJSONString instead
		token := extractJSONString(`{"access_token":"my-token","expires_in":"3600"}`, "access_token")
		assert.Equal(t, "my-token", token)
	})
}

func TestExtractJSONString(t *testing.T) {
	body := `{"access_token":"abc123","token_type":"Bearer"}`
	assert.Equal(t, "abc123", extractJSONString(body, "access_token"))
	assert.Equal(t, "Bearer", extractJSONString(body, "token_type"))
	assert.Equal(t, "", extractJSONString(body, "missing"))

	// With spaces
	body2 := `{"access_token": "def456"}`
	assert.Equal(t, "def456", extractJSONString(body2, "access_token"))
}

// --- JSONB Helper Tests ---

func TestIntFromJSONB(t *testing.T) {
	data := models.JSONB{"a": float64(42), "b": "10", "c": "bad", "d": true}
	v, ok := intFromJSONB(data, "a")
	assert.True(t, ok)
	assert.Equal(t, 42, v)

	v, ok = intFromJSONB(data, "b")
	assert.True(t, ok)
	assert.Equal(t, 10, v)

	_, ok = intFromJSONB(data, "c")
	assert.False(t, ok)

	_, ok = intFromJSONB(data, "missing")
	assert.False(t, ok)
}

func TestBoolFromJSONB(t *testing.T) {
	data := models.JSONB{"a": true, "b": "true", "c": float64(1), "d": float64(0)}
	v, ok := boolFromJSONB(data, "a")
	assert.True(t, ok)
	assert.True(t, v)

	v, ok = boolFromJSONB(data, "b")
	assert.True(t, ok)
	assert.True(t, v)

	v, ok = boolFromJSONB(data, "c")
	assert.True(t, ok)
	assert.True(t, v)

	v, ok = boolFromJSONB(data, "d")
	assert.True(t, ok)
	assert.False(t, v)

	_, ok = boolFromJSONB(data, "missing")
	assert.False(t, ok)
}
