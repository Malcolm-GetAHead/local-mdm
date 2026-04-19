package macos

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/micromdm/nanodep/client"
	"github.com/micromdm/nanodep/godep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDEPProfile(t *testing.T) {
	profile := DefaultDEPProfile("https://mdm.example.com", "Acme Corp")

	assert.Equal(t, "Acme Corp MDM", profile.ProfileName)
	assert.Equal(t, "https://mdm.example.com/mdm", profile.URL)
	assert.True(t, profile.IsSupervised)
	assert.True(t, profile.IsMandatory)
	assert.True(t, profile.AwaitDeviceConfigured)
	assert.False(t, profile.IsMDMRemovable)
	assert.NotEmpty(t, profile.SkipSetupItems)
}

func TestDEPService_SyncDevicesCallbackForName(t *testing.T) {
	// Use a mock storage that tracks calls
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := &DEPService{storage: storage, logger: logger}

	callback := svc.SyncDevicesCallbackForName("test-dep")

	t.Run("handles nil response", func(t *testing.T) {
		err := callback(context.Background(), true, nil)
		assert.NoError(t, err)
	})

	t.Run("stores synced devices", func(t *testing.T) {
		resp := &godep.FetchDeviceResponseJson{
			Devices: []godep.DeviceJson{
				{SerialNumber: "C02ABC123", Model: "MacBookPro18,1"},
				{SerialNumber: "C02DEF456", Model: "MacBookAir10,1"},
			},
		}

		err := callback(context.Background(), true, resp)
		assert.NoError(t, err)
		assert.Len(t, storage.storedDevices, 2)
		assert.Equal(t, "C02ABC123", storage.storedDevices[0].serial)
		assert.Equal(t, "test-dep", storage.storedDevices[0].depName)
	})
}

func TestDEPService_AssignerProfile(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := NewDEPService(storage, logger)

	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		err := svc.SetAssignerProfile(ctx, "test-dep", "profile-uuid-123")
		require.NoError(t, err)

		uuid, modTime, err := svc.GetAssignerProfile(ctx, "test-dep")
		require.NoError(t, err)
		assert.Equal(t, "profile-uuid-123", uuid)
		assert.False(t, modTime.IsZero())
	})
}

func TestDEPService_ListDevices(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := NewDEPService(storage, logger)

	t.Run("defaults limit to 100", func(t *testing.T) {
		devices, total, err := svc.ListDevices(context.Background(), "test", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, devices)
	})
}

func TestDEPService_GenerateTokenPKI(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := NewDEPService(storage, logger)

	certPEM, err := svc.GenerateTokenPKI(context.Background(), "test-dep")
	require.NoError(t, err)
	assert.Contains(t, string(certPEM), "BEGIN CERTIFICATE")
}

func TestDEPService_StoreConfig(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := NewDEPService(storage, logger)

	err := svc.StoreConfig(context.Background(), "test-dep", "https://mdmenrollment.apple.com")
	require.NoError(t, err)
	assert.Equal(t, "https://mdmenrollment.apple.com", storage.configBaseURL)
}

func TestDEPService_SyncCallback_EmptyDevices(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.Default()
	svc := &DEPService{storage: storage, logger: logger}

	callback := svc.SyncDevicesCallbackForName("test")
	err := callback(context.Background(), false, &godep.FetchDeviceResponseJson{
		Devices: []godep.DeviceJson{},
	})
	assert.NoError(t, err)
	assert.Empty(t, storage.storedDevices)
}

func TestDeviceToMap(t *testing.T) {
	d := godep.DeviceJson{
		SerialNumber: "C02TEST123",
		Model:        "MacBookPro18,1",
	}
	m := deviceToMap(d)
	assert.Equal(t, "C02TEST123", m["serial_number"])
	assert.Equal(t, "MacBookPro18,1", m["model"])
}

// --- Mock DEP Storage ---

type storedDevice struct {
	depName string
	serial  string
	data    map[string]interface{}
}

type mockDEPStorage struct {
	storedDevices    []storedDevice
	assignerProfile  string
	assignerModTime  time.Time
	configBaseURL    string
}

func (m *mockDEPStorage) RetrieveAuthTokens(_ context.Context, _ string) (*client.OAuth1Tokens, error) {
	return &client.OAuth1Tokens{}, nil
}
func (m *mockDEPStorage) StoreAuthTokens(_ context.Context, _ string, _ *client.OAuth1Tokens) error {
	return nil
}
func (m *mockDEPStorage) RetrieveConfig(_ context.Context, _ string) (*client.Config, error) {
	return &client.Config{}, nil
}
func (m *mockDEPStorage) StoreConfig(_ context.Context, _ string, cfg *client.Config) error {
	m.configBaseURL = cfg.BaseURL
	return nil
}
func (m *mockDEPStorage) RetrieveCursor(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockDEPStorage) StoreCursor(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *mockDEPStorage) RetrieveAssignerProfile(_ context.Context, _ string) (string, time.Time, error) {
	return m.assignerProfile, m.assignerModTime, nil
}
func (m *mockDEPStorage) StoreAssignerProfile(_ context.Context, _ string, profileUUID string) error {
	m.assignerProfile = profileUUID
	m.assignerModTime = time.Now()
	return nil
}
func (m *mockDEPStorage) GenerateTokenPKI(_ context.Context, _ string, _ string, _ int) ([]byte, error) {
	return []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"), nil
}
func (m *mockDEPStorage) RetrieveCurrentTokenPKI(_ context.Context, _ string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (m *mockDEPStorage) RetrieveStagingTokenPKI(_ context.Context, _ string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (m *mockDEPStorage) UpstageTokenPKI(_ context.Context, _ string) error {
	return nil
}
func (m *mockDEPStorage) StoreSyncedDevice(_ context.Context, depName, serial string, data map[string]interface{}) error {
	m.storedDevices = append(m.storedDevices, storedDevice{depName: depName, serial: serial, data: data})
	return nil
}
func (m *mockDEPStorage) ListDEPDevices(_ context.Context, _ string, _, _ int) ([]DEPDevice, int, error) {
	return nil, 0, nil
}

func TestDEPService_StartDEPSync_Lifecycle(t *testing.T) {
	storage := &mockDEPStorage{}
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewDEPService(storage, logger)

	// Start with a short interval — the sync will fail (mock returns empty tokens)
	// but it should start and stop cleanly without panicking
	cancel := svc.StartDEPSync("test", 100*time.Millisecond)

	// Give the goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel should not panic
	cancel()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
