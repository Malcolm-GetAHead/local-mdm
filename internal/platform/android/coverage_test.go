package android

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/androidmanagement/v1"
)

// mockLifecycleNotifier records OnUnenroll calls.
type mockLifecycleNotifier struct {
	called bool
	device *models.Device
}

func (m *mockLifecycleNotifier) OnUnenroll(_ context.Context, d *models.Device) {
	m.called = true
	m.device = d
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
}

// ─── handleComplianceReport data persistence ────────────────────────────────

func TestHandleComplianceReport_AllFieldsPresent(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterprise := &models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: deviceID},
		PlatformData: models.JSONB{"existing": "value"},
	}

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(enterprise, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(device, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	err := h.handleComplianceReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"compliance": "ok"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", device.PlatformData["compliance"])
	assert.Equal(t, "value", device.PlatformData["existing"])
	deviceRepo.AssertExpectations(t)
}

func TestHandleComplianceReport_EnterpriseNotFound(t *testing.T) {
	ctx := context.Background()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	enterpriseRepo.On("GetBySlug", ctx, "bad").Return(nil, fmt.Errorf("not found"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	err := h.handleComplianceReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "bad",
		Data:            map[string]interface{}{"k": "v"},
	})
	require.NoError(t, err)
}

func TestHandleComplianceReport_DeviceNotFound(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(nil, fmt.Errorf("not found"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	err := h.handleComplianceReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"k": "v"},
	})
	require.NoError(t, err)
}

func TestHandleComplianceReport_UpdateError(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(&models.Device{PlatformData: models.JSONB{}}, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db error"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	err := h.handleComplianceReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"k": "v"},
	})
	require.Error(t, err)
}

func TestHandleComplianceReport_NilPlatformData(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	device := &models.Device{PlatformData: nil}
	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(device, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	err := h.handleComplianceReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"new_key": "new_val"},
	})
	require.NoError(t, err)
	assert.NotNil(t, device.PlatformData)
	assert.Equal(t, "new_val", device.PlatformData["new_key"])
}

// ─── handleStatusReport data persistence ────────────────────────────────────

func TestHandleStatusReport_PersistsData_NoClient(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	device := &models.Device{PlatformData: models.JSONB{}}
	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(device, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, client: nil, logger: quietLogger()}

	err := h.handleStatusReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"battery": "80%"},
	})
	require.NoError(t, err)
	assert.Equal(t, "80%", device.PlatformData["battery"])
}

func TestHandleStatusReport_EnterpriseNotFound_NoClient(t *testing.T) {
	ctx := context.Background()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	enterpriseRepo.On("GetBySlug", ctx, "bad").Return(nil, fmt.Errorf("not found"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, client: nil, logger: quietLogger()}

	err := h.handleStatusReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "bad",
		Data:            map[string]interface{}{"k": "v"},
	})
	require.NoError(t, err)
}

func TestHandleStatusReport_DeviceNotFound_NoClient(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(nil, fmt.Errorf("not found"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, client: nil, logger: quietLogger()}

	err := h.handleStatusReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"k": "v"},
	})
	require.NoError(t, err)
}

func TestHandleStatusReport_UpdateError_NoClient(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(&models.Device{PlatformData: models.JSONB{}}, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db error"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, client: nil, logger: quietLogger()}

	err := h.handleStatusReport(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
		Data:            map[string]interface{}{"k": "v"},
	})
	// client is nil so returns nil after logging the update error
	require.NoError(t, err)
}

// ─── HandleWebhook COMPLIANCE_REPORT error path ─────────────────────────────

func TestHandleWebhook_ComplianceReport_UpdateError_Returns500(t *testing.T) {
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterpriseRepo.On("GetBySlug", mock.Anything, "ent").Return(&models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}, nil)
	deviceRepo.On("GetBySerial", mock.Anything, enterpriseID, "enterprises/x/devices/1").Return(&models.Device{PlatformData: models.JSONB{}}, nil)
	deviceRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db error"))

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := NewWebhookHandler(svc, nil, quietLogger())

	event := WebhookEvent{
		NotificationType: "COMPLIANCE_REPORT",
		DeviceName:       "enterprises/x/devices/1",
		EnterpriseToken:  "ent",
		Timestamp:        "2026-04-17T00:00:00Z",
		Data:             map[string]interface{}{"k": "v"},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleWebhook(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── SetLifecycle + handleUnenrollment lifecycle path ────────────────────────

func TestSetLifecycle_UnenrollmentCallsOnUnenroll(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterprise := &models.Enterprise{BaseModel: models.BaseModel{ID: enterpriseID}}
	device := &models.Device{
		BaseModel: models.BaseModel{ID: deviceID},
		Status:    models.DeviceStatusEnrolled,
	}

	enterpriseRepo.On("GetBySlug", ctx, "ent").Return(enterprise, nil)
	deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/x/devices/1").Return(device, nil)
	deviceRepo.On("GetByID", ctx, deviceID).Return(device, nil)
	deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	h := &WebhookHandler{service: svc, logger: quietLogger()}

	lc := &mockLifecycleNotifier{}
	h.SetLifecycle(lc)

	err := h.handleUnenrollment(ctx, &WebhookEvent{
		DeviceName:      "enterprises/x/devices/1",
		EnterpriseToken: "ent",
	})
	require.NoError(t, err)
	assert.True(t, lc.called)
	assert.Equal(t, deviceID, lc.device.ID)
}

// ─── QR code WiFi SSID without password ─────────────────────────────────────

func TestGenerateQRCode_SSIDWithoutPassword(t *testing.T) {
	qr, err := GenerateQRCode("tok", "https://example.com", "MySSID", "")
	require.NoError(t, err)
	assert.NotEmpty(t, qr)
}

// ─── Constructors ───────────────────────────────────────────────────────────

func TestNewAppManager(t *testing.T) {
	svc, _ := androidmanagement.NewService(context.Background())
	am := NewAppManager(svc)
	require.NotNil(t, am)
	assert.Equal(t, svc, am.service)
}

func TestNewDeviceCommander(t *testing.T) {
	svc, _ := androidmanagement.NewService(context.Background())
	dc := NewDeviceCommander(svc)
	require.NotNil(t, dc)
	assert.Equal(t, svc, dc.service)
}

func TestCreateEnterprise(t *testing.T) {
	client := &Client{logger: quietLogger()}
	ent, err := client.CreateEnterprise(context.Background(), "enterprises/test", "https://signup.example.com")
	require.NoError(t, err)
	assert.Equal(t, "enterprises/test", ent.Name)
	assert.Contains(t, ent.EnabledNotificationTypes, "ENROLLMENT")
}

// ─── Google API wrappers ────────────────────────────────────────────────────

func TestGetEnterprise_Success(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test"})
	}))
	ent, err := client.GetEnterprise(context.Background(), "enterprises/test")
	require.NoError(t, err)
	assert.Equal(t, "enterprises/test", ent.Name)
}

func TestGetEnterprise_Error(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	_, err := client.GetEnterprise(context.Background(), "enterprises/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get enterprise")
}

func TestCreateEnrollmentToken_Success(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":  "enterprises/test/enrollmentTokens/tok1",
			"value": "token-value",
		})
	}))
	tok, err := client.CreateEnrollmentToken(context.Background(), "enterprises/test", "enterprises/test/policies/default")
	require.NoError(t, err)
	assert.Equal(t, "enterprises/test/enrollmentTokens/tok1", tok.Name)
}

func TestCreateEnrollmentToken_Error(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	_, err := client.CreateEnrollmentToken(context.Background(), "enterprises/test", "pol")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create enrollment token")
}

func TestCreatePolicy_Success(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	pol, err := client.CreatePolicy(context.Background(), "enterprises/test", "p1", &androidmanagement.Policy{})
	require.NoError(t, err)
	assert.Equal(t, "enterprises/test/policies/p1", pol.Name)
}

func TestCreatePolicy_Error(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	_, err := client.CreatePolicy(context.Background(), "enterprises/test", "p1", &androidmanagement.Policy{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create policy")
}

func TestGetPolicy_Success(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	pol, err := client.GetPolicy(context.Background(), "enterprises/test/policies/p1")
	require.NoError(t, err)
	assert.Equal(t, "enterprises/test/policies/p1", pol.Name)
}

func TestGetPolicy_Error(t *testing.T) {
	client := newMockClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	_, err := client.GetPolicy(context.Background(), "enterprises/test/policies/p1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get policy")
}

// ─── Device commands ────────────────────────────────────────────────────────

func newMockCommander(t *testing.T, handler http.Handler) *DeviceCommander {
	t.Helper()
	c := newMockClient(t, handler)
	return &DeviceCommander{service: c.service}
}

func commandSuccessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"type": "LOCK"})
	})
}

func commandErrorHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
}

func TestLockDevice_Success(t *testing.T) {
	dc := newMockCommander(t, commandSuccessHandler())
	err := dc.LockDevice(context.Background(), "enterprises/test/devices/d1")
	require.NoError(t, err)
}

func TestLockDevice_Error(t *testing.T) {
	dc := newMockCommander(t, commandErrorHandler())
	err := dc.LockDevice(context.Background(), "enterprises/test/devices/d1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lock device")
}

func TestWipeDevice_FullWipe(t *testing.T) {
	dc := newMockCommander(t, commandSuccessHandler())
	err := dc.WipeDevice(context.Background(), "enterprises/test/devices/d1", false)
	require.NoError(t, err)
}

func TestWipeDevice_WorkProfileOnly(t *testing.T) {
	dc := newMockCommander(t, commandSuccessHandler())
	err := dc.WipeDevice(context.Background(), "enterprises/test/devices/d1", true)
	require.NoError(t, err)
}

func TestWipeDevice_Error(t *testing.T) {
	dc := newMockCommander(t, commandErrorHandler())
	err := dc.WipeDevice(context.Background(), "enterprises/test/devices/d1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to wipe device")
}

func TestRebootDevice_Success(t *testing.T) {
	dc := newMockCommander(t, commandSuccessHandler())
	err := dc.RebootDevice(context.Background(), "enterprises/test/devices/d1")
	require.NoError(t, err)
}

func TestRebootDevice_Error(t *testing.T) {
	dc := newMockCommander(t, commandErrorHandler())
	err := dc.RebootDevice(context.Background(), "enterprises/test/devices/d1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reboot device")
}

// ─── App management ─────────────────────────────────────────────────────────

func newMockAppManager(t *testing.T, handler http.Handler) *AppManager {
	t.Helper()
	c := newMockClient(t, handler)
	return &AppManager{service: c.service}
}

func TestDeployApp_ExistingAppUpdate(t *testing.T) {
	reqCount := 0
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "policies") && r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "enterprises/test/policies/p1",
				"applications": []interface{}{
					map[string]interface{}{
						"packageName": "com.example.app",
						"installType": "AVAILABLE",
					},
				},
			})
			return
		}
		// PATCH
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	err := am.DeployApp(context.Background(), "enterprises/test/policies/p1", "com.example.app", "FORCE_INSTALLED")
	require.NoError(t, err)
}

func TestDeployApp_NewApp(t *testing.T) {
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":         "enterprises/test/policies/p1",
				"applications": []interface{}{},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	err := am.DeployApp(context.Background(), "enterprises/test/policies/p1", "com.new.app", "")
	require.NoError(t, err)
}

func TestDeployApp_GetError(t *testing.T) {
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	err := am.DeployApp(context.Background(), "enterprises/test/policies/p1", "com.example.app", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get policy")
}

func TestRemoveApp_AppPresent(t *testing.T) {
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "enterprises/test/policies/p1",
				"applications": []interface{}{
					map[string]interface{}{"packageName": "com.remove.me"},
					map[string]interface{}{"packageName": "com.keep.me"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	err := am.RemoveApp(context.Background(), "enterprises/test/policies/p1", "com.remove.me")
	require.NoError(t, err)
}

func TestRemoveApp_AppNotPresent(t *testing.T) {
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":         "enterprises/test/policies/p1",
				"applications": []interface{}{},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "enterprises/test/policies/p1"})
	}))
	err := am.RemoveApp(context.Background(), "enterprises/test/policies/p1", "com.nonexistent")
	require.NoError(t, err)
}

func TestRemoveApp_Error(t *testing.T) {
	am := newMockAppManager(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	err := am.RemoveApp(context.Background(), "enterprises/test/policies/p1", "com.example.app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get policy")
}
