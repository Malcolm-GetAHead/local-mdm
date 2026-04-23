package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/android"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
	"github.com/malcolm-getahead/local-mdm/internal/service"
	"github.com/malcolm-getahead/local-mdm/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_MacOSWebhook simulates NanoMDM forwarding an Authenticate webhook
// and verifies a device record is created in the database.
func TestE2E_MacOSWebhook(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()
	logger := slog.Default()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create enterprise
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "macOS Webhook Test",
		Slug:      "macos-webhook-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))

	// Create macOS service and webhook handler
	macosService := macos.NewService(deviceRepo)
	lifecycleSvc := service.NewLifecycleService(logger)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	nanomdmSvc := macos.NewNanoMDMService("http://localhost:9000", "test-key", cmdRepo, deviceRepo, logger)
	checkinHandler := macos.NewCheckinHandler(nanomdmSvc, macosService, lifecycleSvc, logger)

	// Simulate NanoMDM Authenticate webhook
	udid := uuid.New().String()
	webhookEvent := map[string]interface{}{
		"topic":    "com.example.mdm",
		"event_id": uuid.New().String(),
		"checkin_event": map[string]interface{}{
			"udid":         udid,
			"message_type": "Authenticate",
			"params": map[string]interface{}{
				"enterprise_id": enterprise.ID.String(),
			},
		},
	}
	body, _ := json.Marshal(webhookEvent)
	req := httptest.NewRequest("PUT", "/api/v1/macos/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	checkinHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device was created
	devices, total, err := deviceRepo.List(ctx, enterprise.ID, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)

	found := false
	for _, d := range devices {
		if d.DeviceID == udid {
			found = true
			assert.Equal(t, models.PlatformMacOS, d.Platform)
			assert.Equal(t, "pending", d.Status) // Authenticate = pending, TokenUpdate = enrolled
			break
		}
	}
	assert.True(t, found, "device with UDID %s should exist", udid)
}

// TestE2E_MacOSCheckOut simulates a CheckOut webhook and verifies
// the device status is updated to unenrolled.
func TestE2E_MacOSCheckOut(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()
	logger := slog.Default()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create enterprise and device
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "macOS CheckOut Test",
		Slug:      "macos-checkout-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))

	udid := uuid.New().String()
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: "C02TEST" + uuid.New().String()[:4],
		Status:       models.DeviceStatusEnrolled,
	}
	require.NoError(t, deviceRepo.Create(ctx, device))

	// Simulate CheckOut webhook
	macosService := macos.NewService(deviceRepo)
	lifecycleSvc := service.NewLifecycleService(logger)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	nanomdmSvc := macos.NewNanoMDMService("http://localhost:9000", "test-key", cmdRepo, deviceRepo, logger)
	checkinHandler := macos.NewCheckinHandler(nanomdmSvc, macosService, lifecycleSvc, logger)

	webhookEvent := map[string]interface{}{
		"topic":    "com.example.mdm",
		"event_id": uuid.New().String(),
		"checkin_event": map[string]interface{}{
			"udid":         udid,
			"message_type": "CheckOut",
		},
	}
	body, _ := json.Marshal(webhookEvent)
	req := httptest.NewRequest("PUT", "/api/v1/macos/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	checkinHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device status updated
	updated, err := deviceRepo.GetByID(ctx, device.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceStatusUnenrolled, updated.Status)
}

// TestE2E_AndroidWebhookEnrollment simulates an Android enrollment webhook
// and verifies a device record is created.
func TestE2E_AndroidWebhookEnrollment(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()
	logger := slog.Default()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create enterprise
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Android Webhook Test",
		Slug:      "android-webhook-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))

	// Create Android service and webhook handler (no Google client)
	androidService := android.NewService(deviceRepo, entRepo, "", "")
	webhookHandler := android.NewWebhookHandler(androidService, nil, logger)

	// Simulate enrollment webhook
	deviceName := "enterprises/test/devices/" + uuid.New().String()
	event := android.WebhookEvent{
		NotificationType: "ENROLLMENT",
		EnterpriseToken:  enterprise.Slug,
		DeviceName:       deviceName,
		Timestamp:        time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/api/v1/android/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	webhookHandler.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device was created
	devices, _, err := deviceRepo.List(ctx, enterprise.ID, 100, 0)
	require.NoError(t, err)

	found := false
	for _, d := range devices {
		if d.Platform == models.PlatformAndroid {
			found = true
			break
		}
	}
	assert.True(t, found, "Android device should exist after enrollment webhook")
}

// TestE2E_AndroidWebhookUnenrollment simulates an Android unenrollment webhook
// and verifies the device status is updated.
func TestE2E_AndroidWebhookUnenrollment(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()
	logger := slog.Default()

	entRepo, err := repository.NewEnterpriseRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	// Create enterprise and device
	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Android Unenroll Test",
		Slug:      "android-unenroll-" + uuid.New().String()[:8],
	}
	require.NoError(t, entRepo.Create(ctx, enterprise))

	deviceName := "enterprises/test/devices/" + uuid.New().String()
	device := &models.Device{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformAndroid,
		DeviceID:     uuid.New().String(),
		SerialNumber: deviceName,
		Status:       models.DeviceStatusEnrolled,
	}
	require.NoError(t, deviceRepo.Create(ctx, device))

	// Simulate unenrollment webhook
	androidService := android.NewService(deviceRepo, entRepo, "", "")
	webhookHandler := android.NewWebhookHandler(androidService, nil, logger)

	event := android.WebhookEvent{
		NotificationType: "UNENROLLMENT",
		EnterpriseToken:  enterprise.Slug,
		DeviceName:       deviceName,
		Timestamp:        time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/api/v1/android/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	webhookHandler.HandleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device status updated
	updated, err := deviceRepo.GetByID(ctx, device.ID)
	require.NoError(t, err)
	assert.Equal(t, "unenrolled", updated.Status)
}
