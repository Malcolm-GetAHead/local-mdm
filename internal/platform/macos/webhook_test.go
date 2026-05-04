package macos

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testNanoMDMService(t *testing.T) *NanoMDMService {
	t.Helper()
	return NewNanoMDMService("", "", nil, nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})),
	)
}

func testMacOSService(t *testing.T) *Service {
	t.Helper()
	repo := &MockDeviceRepository{}
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	repo.On("GetByPlatformID", mock.Anything, mock.Anything, mock.Anything).Return(&models.Device{
		BaseModel: models.BaseModel{ID: [16]byte{1}},
		Status:    "pending",
	}, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)
	return &Service{deviceRepo: repo}
}

func strPtr(s string) *string { return &s }

func TestCheckinHandler_Authenticate(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	udidStr := "AAAA-BBBB-CCCC"
	event := WebhookEvent{
		Topic:   "mdm.Authenticate",
		EventID: strPtr("evt-1"),
		CheckinEvent: &CheckinEvent{
			UDID:       &udidStr,
			RawPayload: `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>MessageType</key><string>Authenticate</string><key>UDID</key><string>AAAA-BBBB-CCCC</string><key>SerialNumber</key><string>TEST123</string></dict></plist>`,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckinHandler_TokenUpdate(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	udidStr2 := "AAAA-BBBB-CCCC"
	event := WebhookEvent{
		Topic: "mdm.TokenUpdate",
		CheckinEvent: &CheckinEvent{
			UDID:       &udidStr2,
			RawPayload: `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>MessageType</key><string>TokenUpdate</string><key>UDID</key><string>AAAA-BBBB-CCCC</string><key>PushMagic</key><string>test-magic</string></dict></plist>`,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckinHandler_InvalidJSON(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckinHandler_NoCheckinEvent(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := WebhookEvent{Topic: "mdm.Connect"}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCommandHandler_ServeHTTP_Basic(t *testing.T) {
	h := NewCommandHandler(testNanoMDMService(t),
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	req := httptest.NewRequest("PUT", "/mdm", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- processCommandResult Tests ---

// b64Plist encodes a plist XML string to base64 (simulating NanoMDM webhook payload).
func b64Plist(plistXML string) string {
	return base64.StdEncoding.EncodeToString([]byte(plistXML))
}

// newTestCheckinHandler creates a CheckinHandler with a mock device repo that returns
// a device with the given UDID and captures updates.
func newTestCheckinHandler(t *testing.T, udid string, device *models.Device) (*CheckinHandler, *MockDeviceRepository, *MockCommandRepository) {
	t.Helper()
	repo := &MockDeviceRepository{}
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(device, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	cmdRepo := &MockCommandRepository{}
	cmdRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	cmdRepo.On("MarkSent", mock.Anything, mock.Anything).Return(nil)
	cmdRepo.On("MarkCompleted", mock.Anything, mock.Anything).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, cmdRepo, nil, logger)
	return h, repo, cmdRepo
}

func TestProcessCommandResult_SecurityInfo(t *testing.T) {
	udid := "SEC-INFO-TEST-UDID"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{1}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, repo, _ := newTestCheckinHandler(t, udid, device)

	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key>
	<string>cmd-sec-001</string>
	<key>Status</key>
	<string>Acknowledged</string>
	<key>SecurityInfo</key>
	<dict>
		<key>FDE_Enabled</key>
		<true/>
		<key>FirewallSettings</key>
		<dict>
			<key>FirewallEnabled</key>
			<false/>
			<key>StealthMode</key>
			<true/>
		</dict>
		<key>AuthenticatedRootVolumeEnabled</key>
		<true/>
		<key>IsActivationLockManageable</key>
		<false/>
		<key>ExternalBootLevel</key>
		<string>allowed</string>
		<key>SecureBoot</key>
		<string>full</string>
	</dict>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	// Verify device was updated
	repo.AssertCalled(t, "Update", mock.Anything, mock.Anything)

	assert.Equal(t, true, device.PlatformData["FileVaultEnabled"])
	assert.Equal(t, true, device.PlatformData["encryption_enabled"])
	assert.Equal(t, false, device.PlatformData["firewall_enabled"])
	assert.Equal(t, true, device.PlatformData["authenticated_root_volume"])
	assert.Equal(t, false, device.PlatformData["activation_lock_manageable"])
	assert.Equal(t, "allowed", device.PlatformData["external_boot_level"])
	assert.Equal(t, "full", device.PlatformData["secure_boot"])
}

func TestProcessCommandResult_SecurityInfo_FirewallNoEnabled(t *testing.T) {
	udid := "SEC-FW-TEST"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{2}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, _, _ := newTestCheckinHandler(t, udid, device)

	// FirewallSettings present but no FirewallEnabled key → defaults to true
	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-fw</string>
	<key>Status</key><string>Acknowledged</string>
	<key>SecurityInfo</key>
	<dict>
		<key>FirewallSettings</key>
		<dict>
			<key>StealthMode</key><true/>
		</dict>
	</dict>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)
	assert.Equal(t, true, device.PlatformData["firewall_enabled"])
}

func TestProcessCommandResult_DeviceInformation(t *testing.T) {
	udid := "DEV-INFO-TEST-UDID"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{3}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, repo, _ := newTestCheckinHandler(t, udid, device)

	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-devinfo-001</string>
	<key>Status</key><string>Acknowledged</string>
	<key>QueryResponses</key>
	<dict>
		<key>DeviceName</key><string>Test MacBook</string>
		<key>OSVersion</key><string>26.2</string>
		<key>BuildVersion</key><string>25F79</string>
		<key>ModelName</key><string>MacBook Pro</string>
		<key>SerialNumber</key><string>ZL9QG3C3RR</string>
		<key>WiFiMAC</key><string>AA:BB:CC:DD:EE:FF</string>
		<key>BluetoothMAC</key><string>11:22:33:44:55:66</string>
		<key>IsSupervised</key><false/>
		<key>DeviceCapacity</key><real>494.38</real>
		<key>AvailableDeviceCapacity</key><real>412.5</real>
		<key>BatteryLevel</key><real>0.85</real>
		<key>HostName</key><string>test-macbook.local</string>
		<key>TimeZone</key><string>America/New_York</string>
		<key>ProductName</key><string>Mac16,1</string>
		<key>HasBattery</key><true/>
		<key>AutomaticCheckEnabled</key><true/>
		<key>AutomaticOSInstallationEnabled</key><false/>
		<key>AutomaticSecurityUpdatesEnabled</key><true/>
		<key>DiagnosticSubmissionEnabled</key><false/>
		<key>AppAnalyticsEnabled</key><false/>
	</dict>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	repo.AssertCalled(t, "Update", mock.Anything, mock.Anything)

	// Top-level device fields
	assert.Equal(t, "Test MacBook", device.Name)
	assert.Equal(t, "26.2", device.OSVersion)
	assert.Equal(t, "MacBook Pro", device.Model)
	assert.Equal(t, "ZL9QG3C3RR", device.SerialNumber)

	// PlatformData fields
	assert.Equal(t, "25F79", device.PlatformData["build_version"])
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", device.PlatformData["wifi_mac"])
	assert.Equal(t, "11:22:33:44:55:66", device.PlatformData["bluetooth_mac"])
	assert.Equal(t, false, device.PlatformData["is_supervised"])
	assert.Equal(t, "test-macbook.local", device.PlatformData["hostname"])
	assert.Equal(t, "America/New_York", device.PlatformData["timezone"])
	assert.Equal(t, "Mac16,1", device.PlatformData["product_name"])
	assert.Equal(t, true, device.PlatformData["has_battery"])
	assert.Equal(t, true, device.PlatformData["auto_update_check"])
	assert.Equal(t, false, device.PlatformData["auto_os_install"])
	assert.Equal(t, true, device.PlatformData["auto_security_updates"])
	assert.Equal(t, false, device.PlatformData["diagnostics_enabled"])
	assert.Equal(t, false, device.PlatformData["app_analytics_enabled"])
	// Numeric values from plist
	assert.NotNil(t, device.PlatformData["storage_capacity_gb"])
	assert.NotNil(t, device.PlatformData["storage_available_gb"])
	assert.NotNil(t, device.PlatformData["battery_level"])
}

func TestProcessCommandResult_ProfileList(t *testing.T) {
	udid := "PROFILE-LIST-TEST"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{4}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, _, _ := newTestCheckinHandler(t, udid, device)

	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-profiles</string>
	<key>Status</key><string>Acknowledged</string>
	<key>ProfileList</key>
	<array>
		<dict>
			<key>PayloadDisplayName</key><string>MDM Profile</string>
			<key>PayloadIdentifier</key><string>com.localmdm.mdm</string>
			<key>PayloadOrganization</key><string>Acme Corp</string>
			<key>PayloadUUID</key><string>uuid-1</string>
			<key>IsManaged</key><true/>
		</dict>
	</array>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	assert.Equal(t, 1, device.PlatformData["installed_profiles_count"])
	profiles := device.PlatformData["installed_profiles"].([]map[string]interface{})
	assert.Equal(t, "MDM Profile", profiles[0]["name"])
	assert.Equal(t, "com.localmdm.mdm", profiles[0]["identifier"])
}

func TestProcessCommandResult_CertificateList(t *testing.T) {
	udid := "CERT-LIST-TEST"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{5}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, _, _ := newTestCheckinHandler(t, udid, device)

	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-certs</string>
	<key>Status</key><string>Acknowledged</string>
	<key>CertificateList</key>
	<array>
		<dict>
			<key>CommonName</key><string>LocalMDM CA</string>
			<key>IsIdentity</key><false/>
		</dict>
		<dict>
			<key>CommonName</key><string>MDM Device Cert</string>
			<key>IsIdentity</key><true/>
		</dict>
	</array>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	assert.Equal(t, 2, device.PlatformData["certificates_count"])
	certs := device.PlatformData["certificates"].([]map[string]interface{})
	assert.Equal(t, "LocalMDM CA", certs[0]["common_name"])
	assert.Equal(t, true, certs[1]["is_identity"])
}

func TestProcessCommandResult_UserList(t *testing.T) {
	udid := "USER-LIST-TEST"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{6}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, _, _ := newTestCheckinHandler(t, udid, device)

	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-users</string>
	<key>Status</key><string>Acknowledged</string>
	<key>Users</key>
	<array>
		<dict>
			<key>UserName</key><string>testuser</string>
			<key>FullName</key><string>Test User</string>
			<key>UID</key><integer>501</integer>
			<key>IsAdmin</key><true/>
			<key>HasSecureToken</key><true/>
		</dict>
	</array>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	assert.Equal(t, 1, device.PlatformData["local_users_count"])
	users := device.PlatformData["local_users"].([]map[string]interface{})
	assert.Equal(t, "testuser", users[0]["username"])
	assert.Equal(t, true, users[0]["is_admin"])
}

func TestProcessCommandResult_InvalidBase64(t *testing.T) {
	udid := "INVALID-B64"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{7}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, repo, _ := newTestCheckinHandler(t, udid, device)

	h.processCommandResult(context.Background(), udid, "not-valid-base64!!!")

	// Should not crash, should not update device
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProcessCommandResult_InvalidPlist(t *testing.T) {
	udid := "INVALID-PLIST"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{8}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, repo, _ := newTestCheckinHandler(t, udid, device)

	payload := base64.StdEncoding.EncodeToString([]byte("this is not a plist"))
	h.processCommandResult(context.Background(), udid, payload)

	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProcessCommandResult_EmptyResult(t *testing.T) {
	udid := "EMPTY-RESULT"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{9}},
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, repo, _ := newTestCheckinHandler(t, udid, device)

	// Valid plist but no SecurityInfo/QueryResponses/etc
	payload := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CommandUUID</key><string>cmd-empty</string>
	<key>Status</key><string>Acknowledged</string>
</dict>
</plist>`)

	h.processCommandResult(context.Background(), udid, payload)

	// No data to update — Update should not be called
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// --- MockCommandRepository ---

type MockCommandRepository struct {
	mock.Mock
}

func (m *MockCommandRepository) Create(ctx context.Context, cmd *models.DeviceCommand) error {
	args := m.Called(ctx, cmd)
	return args.Error(0)
}

func (m *MockCommandRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DeviceCommand, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeviceCommand), args.Error(1)
}

func (m *MockCommandRepository) ListPending(ctx context.Context, deviceID uuid.UUID) ([]*models.DeviceCommand, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DeviceCommand), args.Error(1)
}

func (m *MockCommandRepository) ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*models.DeviceCommand, int, error) {
	args := m.Called(ctx, deviceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.DeviceCommand), args.Int(1), args.Error(2)
}

func (m *MockCommandRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCommandRepository) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCommandRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

// --- maybeAutoQueue Tests ---

func TestMaybeAutoQueue_Cooldown(t *testing.T) {
	udid := "COOLDOWN-TEST-UDID"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{10}},
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		DeviceID:     udid,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	h, _, cmdRepo := newTestCheckinHandler(t, udid, device)

	ctx := context.Background()

	// First call should queue commands: Create (pending) then MarkSent for each
	h.maybeAutoQueue(ctx, udid)
	assert.True(t, cmdRepo.AssertNumberOfCalls(t, "Create", 9))
	assert.True(t, cmdRepo.AssertNumberOfCalls(t, "MarkSent", 9))

	// Verify commands are created with pending status
	for _, call := range cmdRepo.Calls {
		if call.Method == "Create" {
			cmd := call.Arguments.Get(1).(*models.DeviceCommand)
			assert.Equal(t, models.CommandStatusPending, cmd.Status, "command should be created as pending")
			assert.Nil(t, cmd.SentAt, "SentAt should be nil on creation")
		}
	}

	// Second call within cooldown should NOT queue
	cmdRepo.Calls = nil // reset call tracking
	h.maybeAutoQueue(ctx, udid)
	cmdRepo.AssertNumberOfCalls(t, "Create", 0)

	// Simulate cooldown elapsed by manipulating lastAutoQuery
	h.autoQueryMu.Lock()
	h.lastAutoQuery[udid] = time.Now().Add(-16 * time.Minute)
	h.autoQueryMu.Unlock()

	// Third call after cooldown should queue again
	h.maybeAutoQueue(ctx, udid)
	cmdRepo.AssertNumberOfCalls(t, "Create", 9)
	cmdRepo.AssertNumberOfCalls(t, "MarkSent", 9)
}

// --- Integration-style webhook flow tests ---
// These verify the full data flow through the HTTP handler, checking that
// device records are created/updated with the correct fields from plist payloads.

func TestWebhookFlow_Authenticate_CreatesDevice(t *testing.T) {
	// Simulate: NanoMDM sends Authenticate webhook for a soft-deleted device → device restored
	udid := "FLOW-AUTH-UDID"
	enterpriseID := uuid.New()
	var updatedDevice *models.Device

	repo := &MockDeviceRepository{}
	// First call: device not found in active records
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(nil, assert.AnError).Once()
	// Soft-deleted record found — provides the correct enterprise ID
	deletedDevice := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EnterpriseID: enterpriseID,
		DeviceID:     udid,
		Platform:     "macos",
	}
	repo.On("GetByPlatformIDIncludeDeleted", mock.Anything, "macos", udid).Return(deletedDevice, nil)
	// Create succeeds (restores the soft-deleted device)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.Device")).Return(nil)
	// After create, return a device for subsequent lookups (last_seen update)
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(&models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EnterpriseID: enterpriseID,
		DeviceID:     udid,
		Platform:     "macos",
	}, nil)
	// Capture the device passed to Update (this is where the plist fields land)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(d *models.Device) bool {
		if d.DeviceID == udid && d.Name != "" {
			updatedDevice = d
		}
		return true
	})).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, nil, nil, logger)

	// Authenticate plist with full device info
	plistPayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>MessageType</key><string>Authenticate</string>
	<key>UDID</key><string>FLOW-AUTH-UDID</string>
	<key>SerialNumber</key><string>ZL9QG3C3RR</string>
	<key>DeviceName</key><string>Malcolm's MacBook Pro</string>
	<key>ModelName</key><string>MacBook Pro</string>
	<key>OSVersion</key><string>26.2</string>
	<key>BuildVersion</key><string>25F79</string>
	<key>Topic</key><string>com.apple.mgmt.External.test</string>
</dict></plist>`

	event := WebhookEvent{
		Topic: "mdm.Authenticate",
		CheckinEvent: &CheckinEvent{
			UDID:       &udid,
			RawPayload: plistPayload,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device was restored and updated with plist data
	require.NotNil(t, updatedDevice)
	assert.Equal(t, udid, updatedDevice.DeviceID)
	assert.Equal(t, "ZL9QG3C3RR", updatedDevice.SerialNumber)
	assert.Equal(t, "Malcolm's MacBook Pro", updatedDevice.Name)
	assert.Equal(t, "MacBook Pro", updatedDevice.Model)
	assert.Equal(t, "26.2", updatedDevice.OSVersion)
	assert.Equal(t, "25F79", updatedDevice.PlatformData["build_version"])
	assert.Equal(t, "com.apple.mgmt.External.test", updatedDevice.PlatformData["topic"])
}

func TestWebhookFlow_Authenticate_UnknownDevice_Rejected(t *testing.T) {
	// Simulate: NanoMDM sends Authenticate for a device with no record at all → rejected
	udid := "FLOW-UNKNOWN-UDID"

	repo := &MockDeviceRepository{}
	// Not found in active records
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(nil, assert.AnError)
	// Not found in deleted records either
	repo.On("GetByPlatformIDIncludeDeleted", mock.Anything, "macos", udid).Return(nil, assert.AnError)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, nil, nil, logger)

	plistPayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>MessageType</key><string>Authenticate</string>
	<key>UDID</key><string>FLOW-UNKNOWN-UDID</string>
</dict></plist>`

	event := WebhookEvent{
		Topic: "mdm.Authenticate",
		CheckinEvent: &CheckinEvent{
			UDID:       &udid,
			RawPayload: plistPayload,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify no device was created — Create should NOT be called
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestWebhookFlow_TokenUpdate_SetsEnrolled(t *testing.T) {
	// Simulate: NanoMDM sends TokenUpdate webhook → device status set to enrolled, push_magic stored
	udid := "FLOW-TOKEN-UDID"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		DeviceID:     udid,
		Platform:     "macos",
		Status:       models.DeviceStatusPending,
		PlatformData: models.JSONB{},
	}

	repo := &MockDeviceRepository{}
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(device, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, nil, nil, logger)

	plistPayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>MessageType</key><string>TokenUpdate</string>
	<key>UDID</key><string>FLOW-TOKEN-UDID</string>
	<key>PushMagic</key><string>AAAA-BBBB-CCCC-DDDD</string>
	<key>Token</key><data>dGVzdC10b2tlbg==</data>
</dict></plist>`

	event := WebhookEvent{
		Topic: "mdm.TokenUpdate",
		CheckinEvent: &CheckinEvent{
			UDID:       &udid,
			RawPayload: plistPayload,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device status and push_magic
	assert.Equal(t, models.DeviceStatusEnrolled, device.Status)
	assert.Equal(t, "AAAA-BBBB-CCCC-DDDD", device.PlatformData["push_magic"])
	assert.Equal(t, true, device.PlatformData["has_token"])
}

func TestWebhookFlow_Acknowledge_SecurityInfo(t *testing.T) {
	// Simulate: NanoMDM sends Acknowledge with SecurityInfo → platform_data updated
	udid := "FLOW-ACK-UDID"
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		DeviceID:     udid,
		Platform:     "macos",
		Status:       models.DeviceStatusEnrolled,
		PlatformData: models.JSONB{},
	}

	repo := &MockDeviceRepository{}
	repo.On("GetByPlatformID", mock.Anything, "macos", udid).Return(device, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	cmdRepo := &MockCommandRepository{}
	cmdRepo.On("MarkCompleted", mock.Anything, mock.Anything).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, cmdRepo, nil, logger)

	secInfoPlist := b64Plist(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CommandUUID</key><string>cmd-sec-flow</string>
	<key>Status</key><string>Acknowledged</string>
	<key>SecurityInfo</key><dict>
		<key>FDE_Enabled</key><true/>
		<key>FirewallSettings</key><dict>
			<key>FirewallEnabled</key><true/>
		</dict>
	</dict>
</dict></plist>`)

	cmdUUID := "cmd-sec-flow"
	event := WebhookEvent{
		Topic: "mdm.Acknowledge",
		AcknowledgeEvent: &AcknowledgeEvent{
			UDID:        &udid,
			CommandUUID: &cmdUUID,
			Status:      "Acknowledged",
			RawPayload:  secInfoPlist,
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify platform_data was updated with SecurityInfo
	assert.Equal(t, true, device.PlatformData["FileVaultEnabled"])
	assert.Equal(t, true, device.PlatformData["encryption_enabled"])
	assert.Equal(t, true, device.PlatformData["firewall_enabled"])
}

func TestMaybeAutoQueue_DifferentDevices(t *testing.T) {
	udid1 := "DEVICE-1"
	udid2 := "DEVICE-2"
	device1 := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{11}},
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		DeviceID:     udid1,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}
	device2 := &models.Device{
		BaseModel:    models.BaseModel{ID: [16]byte{12}},
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		DeviceID:     udid2,
		Platform:     "macos",
		PlatformData: models.JSONB{},
	}

	repo := &MockDeviceRepository{}
	repo.On("GetByPlatformID", mock.Anything, "macos", udid1).Return(device1, nil)
	repo.On("GetByPlatformID", mock.Anything, "macos", udid2).Return(device2, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	cmdRepo := &MockCommandRepository{}
	cmdRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	cmdRepo.On("MarkSent", mock.Anything, mock.Anything).Return(nil)

	svc := &Service{deviceRepo: repo}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewCheckinHandler(testNanoMDMService(t), svc, cmdRepo, nil, logger)

	ctx := context.Background()

	// Queue for device 1 — 9 Create + 9 MarkSent = 18 calls
	h.maybeAutoQueue(ctx, udid1)
	assert.Equal(t, 18, len(cmdRepo.Calls))

	// Queue for device 2 — should work (different device, independent cooldown)
	h.maybeAutoQueue(ctx, udid2)
	assert.Equal(t, 36, len(cmdRepo.Calls))
}
