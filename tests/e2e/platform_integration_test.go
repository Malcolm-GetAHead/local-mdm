package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
// for a soft-deleted device and verifies the device record is restored.
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
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

	// Pre-create a device (simulating prior SCEP enrollment), then soft-delete it
	udid := uuid.New().String()
	device := &models.Device{
		EnterpriseID: enterprise.ID,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: "SN" + uuid.New().String()[:8],
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(ctx, device))
	originalID := device.ID
	require.NoError(t, deviceRepo.Delete(ctx, originalID))

	// Create macOS service and webhook handler
	macosService := macos.NewService(deviceRepo)
	lifecycleSvc := service.NewLifecycleService(logger)
	cmdRepo, err := repository.NewCommandRepository(database.Writer, database.Reader)
	require.NoError(t, err)
	nanomdmSvc := macos.NewNanoMDMService("http://localhost:9000", "test-key", cmdRepo, deviceRepo, logger)
	checkinHandler := macos.NewCheckinHandler(nanomdmSvc, macosService, nil, lifecycleSvc, logger)

	// Simulate NanoMDM Authenticate webhook — device should be restored
	authPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>MessageType</key><string>Authenticate</string><key>UDID</key><string>%s</string><key>SerialNumber</key><string>SN-REENROLL</string></dict></plist>`, udid)
	rawPayload := base64.StdEncoding.EncodeToString([]byte(authPlist))
	webhookEvent := map[string]interface{}{
		"topic":    "mdm.Authenticate",
		"event_id": uuid.New().String(),
		"checkin_event": map[string]interface{}{
			"udid":        udid,
			"raw_payload": rawPayload,
		},
	}
	body, _ := json.Marshal(webhookEvent)
	req := httptest.NewRequest("PUT", "/api/v1/macos/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	checkinHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify device was restored with same UUID
	restored, err := deviceRepo.GetByPlatformID(ctx, models.PlatformMacOS, udid)
	require.NoError(t, err)
	assert.Equal(t, originalID, restored.ID, "restored device should have same UUID")
	assert.Equal(t, enterprise.ID, restored.EnterpriseID, "restored device should be in correct enterprise")
	assert.Equal(t, "enrolled", restored.Status)
	assert.Nil(t, restored.DeletedAt)
}

// TestE2E_DeviceReenrollment_FullFlow tests the complete re-enrollment lifecycle:
// create → soft-delete → re-enroll (same enterprise) → verify restoration
// Also verifies a second enterprise can independently enroll the same UDID.
func TestE2E_DeviceReenrollment_FullFlow(t *testing.T) {
	database := testutil.ConnectDB(t)
	ctx := context.Background()

	entID1 := testutil.CreateTestEnterprise(t, database.Writer, "inttest-reenroll-e2e-1")
	entID2 := testutil.CreateTestEnterprise(t, database.Writer, "inttest-reenroll-e2e-2")

	deviceRepo, err := repository.NewDeviceRepository(database.Writer, database.Reader)
	require.NoError(t, err)

	udid := "e2e-reenroll-" + uuid.New().String()[:8]

	// Step 1: Create device in enterprise 1
	device := &models.Device{
		EnterpriseID: entID1,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: "SN-ORIG",
		Name:         "Original Device",
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(ctx, device))
	originalID := device.ID

	// Step 2: Soft-delete the device
	require.NoError(t, deviceRepo.Delete(ctx, originalID))

	// Verify it's gone from normal queries
	_, err = deviceRepo.GetByID(ctx, originalID)
	require.Error(t, err)

	// Step 3: Re-enroll with same enterprise/platform/device_id
	reDevice := &models.Device{
		EnterpriseID: entID1,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: "SN-REENROLL",
		Name:         "Re-enrolled Device",
		Status:       "pending",
		PlatformData: models.JSONB{"reenrolled": true},
	}
	require.NoError(t, deviceRepo.Create(ctx, reDevice))

	// Verify: same UUID restored, correct enterprise, enrolled status
	assert.Equal(t, originalID, reDevice.ID, "re-enrollment should restore original UUID")
	assert.Equal(t, "enrolled", reDevice.Status, "restored device should be enrolled")

	fetched, err := deviceRepo.GetByID(ctx, originalID)
	require.NoError(t, err)
	assert.Equal(t, entID1, fetched.EnterpriseID)
	assert.Equal(t, "SN-REENROLL", fetched.SerialNumber)
	assert.Nil(t, fetched.DeletedAt)

	// Step 4: Second enterprise enrolls the same UDID independently
	d2 := &models.Device{
		EnterpriseID: entID2,
		Platform:     models.PlatformMacOS,
		DeviceID:     udid,
		SerialNumber: "SN-ENT2",
		Status:       "enrolled",
		PlatformData: models.JSONB{},
	}
	require.NoError(t, deviceRepo.Create(ctx, d2))
	assert.NotEqual(t, originalID, d2.ID, "different enterprise should get a new UUID")
	assert.Equal(t, entID2, d2.EnterpriseID)
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
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

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
	checkinHandler := macos.NewCheckinHandler(nanomdmSvc, macosService, nil, lifecycleSvc, logger)

	webhookEvent := map[string]interface{}{
		"topic":    "mdm.CheckOut",
		"event_id": uuid.New().String(),
		"checkin_event": map[string]interface{}{
			"udid":        udid,
			"raw_payload": base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>MessageType</key><string>CheckOut</string><key>UDID</key><string>%s</string></dict></plist>`, udid))),
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
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

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
	t.Cleanup(func() { database.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })

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
