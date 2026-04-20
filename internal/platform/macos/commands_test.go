package macos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Command Builder Tests ---

func TestBuildDeviceInformationCommand(t *testing.T) {
	plist, cmdUUID := BuildDeviceInformationCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "DeviceInformation")
	assert.Contains(t, string(plist), cmdUUID)
	assert.Contains(t, string(plist), "DeviceName")
	assert.Contains(t, string(plist), "OSVersion")
	assert.Contains(t, string(plist), "SerialNumber")
}

func TestBuildSecurityInfoCommand(t *testing.T) {
	plist, cmdUUID := BuildSecurityInfoCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "SecurityInfo")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildProfileListCommand(t *testing.T) {
	plist, cmdUUID := BuildProfileListCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "ProfileList")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildInstalledApplicationListCommand(t *testing.T) {
	plist, cmdUUID := BuildInstalledApplicationListCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "InstalledApplicationList")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildCertificateListCommand(t *testing.T) {
	plist, cmdUUID := BuildCertificateListCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "CertificateList")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildInstallProfileCommand(t *testing.T) {
	profileData := []byte("<plist>test profile</plist>")
	plist, cmdUUID := BuildInstallProfileCommand(profileData)
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "InstallProfile")
	assert.Contains(t, string(plist), cmdUUID)
	assert.Contains(t, string(plist), "<data>")
}

func TestBuildRemoveProfileCommand(t *testing.T) {
	plist, cmdUUID := BuildRemoveProfileCommand("com.example.wifi")
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "RemoveProfile")
	assert.Contains(t, string(plist), "com.example.wifi")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildDeviceLockCommand(t *testing.T) {
	t.Run("with pin and message", func(t *testing.T) {
		plist, cmdUUID := BuildDeviceLockCommand("123456", "Device locked by admin")
		assert.NotEmpty(t, cmdUUID)
		assert.Contains(t, string(plist), "DeviceLock")
		assert.Contains(t, string(plist), "123456")
		assert.Contains(t, string(plist), "Device locked by admin")
	})

	t.Run("without pin", func(t *testing.T) {
		plist, _ := BuildDeviceLockCommand("", "")
		assert.Contains(t, string(plist), "DeviceLock")
		assert.NotContains(t, string(plist), "<key>PIN</key>")
	})
}

func TestBuildEraseDeviceCommand(t *testing.T) {
	t.Run("with pin", func(t *testing.T) {
		plist, cmdUUID := BuildEraseDeviceCommand("654321")
		assert.NotEmpty(t, cmdUUID)
		assert.Contains(t, string(plist), "EraseDevice")
		assert.Contains(t, string(plist), "654321")
	})

	t.Run("without pin", func(t *testing.T) {
		plist, _ := BuildEraseDeviceCommand("")
		assert.Contains(t, string(plist), "EraseDevice")
		assert.NotContains(t, string(plist), "<key>PIN</key>")
	})
}

func TestBuildRestartDeviceCommand(t *testing.T) {
	plist, cmdUUID := BuildRestartDeviceCommand()
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "RestartDevice")
	assert.Contains(t, string(plist), cmdUUID)
}

func TestBuildInstallApplicationCommand(t *testing.T) {
	t.Run("with iTunes store ID", func(t *testing.T) {
		plist, cmdUUID := BuildInstallApplicationCommand(123456, "")
		assert.NotEmpty(t, cmdUUID)
		assert.Contains(t, string(plist), "InstallApplication")
		assert.Contains(t, string(plist), "iTunesStoreID")
		assert.Contains(t, string(plist), "123456")
	})

	t.Run("with manifest URL", func(t *testing.T) {
		plist, _ := BuildInstallApplicationCommand(0, "https://example.com/manifest.plist")
		assert.Contains(t, string(plist), "ManifestURL")
		assert.Contains(t, string(plist), "https://example.com/manifest.plist")
	})
}

func TestBuildRemoveApplicationCommand(t *testing.T) {
	plist, cmdUUID := BuildRemoveApplicationCommand("com.example.app")
	assert.NotEmpty(t, cmdUUID)
	assert.Contains(t, string(plist), "RemoveApplication")
	assert.Contains(t, string(plist), "com.example.app")
}

// --- CommandSender Tests ---

func TestCommandSender_SendCommand(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/enqueue", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			var req EnqueueRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, []string{"test-udid"}, req.UDIDs)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(EnqueueResponse{
				CommandUUID: "cmd-123",
				Status:      map[string]string{"test-udid": "Acknowledged"},
			})
		}))
		defer server.Close()

		sender := NewCommandSender(server.URL, "test-key")
		plist, _ := BuildDeviceInformationCommand()
		resp, err := sender.SendCommand(context.Background(), "test-udid", plist)
		require.NoError(t, err)
		assert.Equal(t, "cmd-123", resp.CommandUUID)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		sender := NewCommandSender(server.URL, "")
		_, err := sender.SendCommand(context.Background(), "test-udid", []byte("test"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("no api key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(EnqueueResponse{CommandUUID: "cmd-456"})
		}))
		defer server.Close()

		sender := NewCommandSender(server.URL, "")
		resp, err := sender.SendCommand(context.Background(), "udid", []byte("test"))
		require.NoError(t, err)
		assert.Equal(t, "cmd-456", resp.CommandUUID)
	})
}

// --- Profile Generation Tests ---

func TestGenerateWiFiProfile(t *testing.T) {
	t.Run("basic WPA2", func(t *testing.T) {
		profile, err := GenerateWiFiProfile(WiFiProfileConfig{
			ProfileConfig: ProfileConfig{
				OrgName:     "TestOrg",
				Description: "Test WiFi",
			},
			SSID:         "CorpNet",
			SecurityType: "WPA2",
			Password:     "secret123",
			AutoJoin:     true,
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "CorpNet")
		assert.Contains(t, s, "WPA2")
		assert.Contains(t, s, "secret123")
		assert.Contains(t, s, "com.apple.wifi.managed")
		assert.Contains(t, s, "TestOrg")
		assert.Contains(t, s, "<true/>") // AutoJoin
	})

	t.Run("hidden network", func(t *testing.T) {
		profile, err := GenerateWiFiProfile(WiFiProfileConfig{
			SSID:     "HiddenNet",
			IsHidden: true,
		})
		require.NoError(t, err)
		assert.Contains(t, string(profile), "HiddenNet")
	})

	t.Run("missing SSID", func(t *testing.T) {
		_, err := GenerateWiFiProfile(WiFiProfileConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SSID is required")
	})

	t.Run("WPA3", func(t *testing.T) {
		profile, err := GenerateWiFiProfile(WiFiProfileConfig{
			SSID:         "SecureNet",
			SecurityType: "WPA3",
		})
		require.NoError(t, err)
		assert.Contains(t, string(profile), "WPA3")
	})

	t.Run("open network", func(t *testing.T) {
		profile, err := GenerateWiFiProfile(WiFiProfileConfig{
			SSID:         "OpenNet",
			SecurityType: "None",
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "None")
		assert.NotContains(t, s, "<key>Password</key>")
	})
}

func TestGenerateVPNProfile(t *testing.T) {
	t.Run("IKEv2", func(t *testing.T) {
		profile, err := GenerateVPNProfile(VPNProfileConfig{
			ProfileConfig: ProfileConfig{
				OrgName: "TestOrg",
			},
			VPNType:       "IKEv2",
			ServerAddress: "vpn.example.com",
			RemoteID:      "vpn.example.com",
			SharedSecret:  "shared-secret",
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "IKEv2")
		assert.Contains(t, s, "vpn.example.com")
		assert.Contains(t, s, "shared-secret")
		assert.Contains(t, s, "com.apple.vpn.managed")
	})

	t.Run("missing server", func(t *testing.T) {
		_, err := GenerateVPNProfile(VPNProfileConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server address is required")
	})

	t.Run("defaults", func(t *testing.T) {
		profile, err := GenerateVPNProfile(VPNProfileConfig{
			ServerAddress: "vpn.test.com",
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "IKEv2") // default type
		assert.Contains(t, s, "com.localmdm.vpn") // default identifier
	})
}

func TestGenerateCertificateProfile(t *testing.T) {
	t.Run("PKCS12", func(t *testing.T) {
		profile, err := GenerateCertificateProfile(CertificateProfileConfig{
			ProfileConfig: ProfileConfig{
				DisplayName: "Corp Cert",
			},
			CertData:   []byte("fake-cert-data"),
			CertFormat: "PKCS12",
			Password:   "cert-pass",
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "com.apple.security.pkcs12")
		assert.Contains(t, s, "cert-pass")
		assert.Contains(t, s, "Corp Cert")
	})

	t.Run("PEM", func(t *testing.T) {
		profile, err := GenerateCertificateProfile(CertificateProfileConfig{
			CertData:   []byte("pem-data"),
			CertFormat: "PEM",
		})
		require.NoError(t, err)
		assert.Contains(t, string(profile), "com.apple.security.pem")
	})

	t.Run("missing cert data", func(t *testing.T) {
		_, err := GenerateCertificateProfile(CertificateProfileConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate data is required")
	})
}

func TestGenerateRestrictionsProfile(t *testing.T) {
	t.Run("all restrictions", func(t *testing.T) {
		bTrue := true
		bFalse := false
		profile, err := GenerateRestrictionsProfile(RestrictionsProfileConfig{
			ProfileConfig: ProfileConfig{
				OrgName: "TestOrg",
			},
			AllowCamera:          &bFalse,
			AllowScreenCapture:   &bFalse,
			AllowAppInstallation: &bTrue,
			ForceEncryption:      &bTrue,
		})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "allowCamera")
		assert.Contains(t, s, "allowScreenCapture")
		assert.Contains(t, s, "allowAppInstallation")
		assert.Contains(t, s, "forceEncryptedBackup")
		assert.Contains(t, s, "com.apple.applicationaccess")
	})

	t.Run("no restrictions set", func(t *testing.T) {
		profile, err := GenerateRestrictionsProfile(RestrictionsProfileConfig{})
		require.NoError(t, err)
		s := string(profile)
		assert.Contains(t, s, "com.apple.applicationaccess")
		assert.NotContains(t, s, "allowCamera")
	})
}

// --- Response Processor Tests ---

func TestResponseProcessor_ProcessCommandResult(t *testing.T) {
	t.Run("acknowledged", func(t *testing.T) {
		cmdRepo := &mockCommandRepo{}
		cmd := createTestCommand(t, cmdRepo, "device_info")
		proc := NewResponseProcessor(cmdRepo, nil, testLogger())

		err := proc.ProcessCommandResult(context.Background(), "test-udid", cmd.ID.String(), "Acknowledged")
		require.NoError(t, err)

		updated, _ := cmdRepo.GetByID(context.Background(), cmd.ID)
		assert.Equal(t, "completed", updated.Status)
	})

	t.Run("error", func(t *testing.T) {
		cmdRepo := &mockCommandRepo{}
		cmd := createTestCommand(t, cmdRepo, "lock")
		proc := NewResponseProcessor(cmdRepo, nil, testLogger())

		err := proc.ProcessCommandResult(context.Background(), "test-udid", cmd.ID.String(), "Error")
		require.NoError(t, err)

		updated, _ := cmdRepo.GetByID(context.Background(), cmd.ID)
		assert.Equal(t, "failed", updated.Status)
		assert.Contains(t, updated.ErrorMessage, "Error")
	})

	t.Run("not now", func(t *testing.T) {
		cmdRepo := &mockCommandRepo{}
		cmd := createTestCommand(t, cmdRepo, "install_profile")
		_ = cmdRepo.MarkSent(context.Background(), cmd.ID)
		proc := NewResponseProcessor(cmdRepo, nil, testLogger())

		err := proc.ProcessCommandResult(context.Background(), "test-udid", cmd.ID.String(), "NotNow")
		require.NoError(t, err)

		updated, _ := cmdRepo.GetByID(context.Background(), cmd.ID)
		assert.Equal(t, "sent", updated.Status) // unchanged
	})

	t.Run("non-uuid command", func(t *testing.T) {
		proc := NewResponseProcessor(&mockCommandRepo{}, nil, testLogger())
		err := proc.ProcessCommandResult(context.Background(), "udid", "not-a-uuid", "Acknowledged")
		require.NoError(t, err) // should not error
	})

	t.Run("command not found", func(t *testing.T) {
		proc := NewResponseProcessor(&mockCommandRepo{}, nil, testLogger())
		err := proc.ProcessCommandResult(context.Background(), "udid", "00000000-0000-0000-0000-000000000099", "Acknowledged")
		require.NoError(t, err) // should not error, just log
	})
}

// --- Helpers ---

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func createTestCommand(t *testing.T, repo *mockCommandRepo, cmdType string) *models.DeviceCommand {
	t.Helper()
	cmd := &models.DeviceCommand{
		DeviceID:    uuid.New(),
		CommandType: cmdType,
	}
	require.NoError(t, repo.Create(context.Background(), cmd))
	return cmd
}

// mockCommandRepo is an in-memory CommandRepository for tests.
type mockCommandRepo struct {
	commands []*models.DeviceCommand
}

func (m *mockCommandRepo) Create(_ context.Context, cmd *models.DeviceCommand) error {
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *mockCommandRepo) GetByID(_ context.Context, id uuid.UUID) (*models.DeviceCommand, error) {
	for _, c := range m.commands {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("command not found")
}

func (m *mockCommandRepo) ListPending(_ context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	var result []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID && c.Status == "pending" {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCommandRepo) ListByDevice(_ context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error) {
	var all []*models.DeviceCommand
	for _, c := range m.commands {
		if c.DeviceID == deviceID {
			all = append(all, c)
		}
	}
	total := len(all)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset >= total {
		return []*models.DeviceCommand{}, total, nil
	}
	return all[offset:end], total, nil
}

func (m *mockCommandRepo) MarkSent(_ context.Context, id uuid.UUID) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "sent"
			return nil
		}
	}
	return fmt.Errorf("command not found")
}

func (m *mockCommandRepo) MarkCompleted(_ context.Context, id uuid.UUID) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "completed"
			return nil
		}
	}
	return fmt.Errorf("command not found")
}

func (m *mockCommandRepo) MarkFailed(_ context.Context, id uuid.UUID, errMsg string) error {
	for _, c := range m.commands {
		if c.ID == id {
			c.Status = "failed"
			c.ErrorMessage = errMsg
			return nil
		}
	}
	return fmt.Errorf("command not found")
}
