package android

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDeviceRepository is a mock implementation of DeviceRepository
type MockDeviceRepository struct {
	mock.Mock
}

func (m *MockDeviceRepository) Create(ctx context.Context, device *models.Device) error {
	args := m.Called(ctx, device)
	if args.Get(0) != nil {
		device.ID = uuid.New()
	}
	return args.Error(0)
}

func (m *MockDeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockDeviceRepository) GetBySerial(ctx context.Context, enterpriseID uuid.UUID, serial string) (*models.Device, error) {
	args := m.Called(ctx, enterpriseID, serial)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockDeviceRepository) Update(ctx context.Context, device *models.Device) error {
	args := m.Called(ctx, device)
	return args.Error(0)
}

func (m *MockDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDeviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
	args := m.Called(ctx, enterpriseID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Device), args.Int(1), args.Error(2)
}

// MockEnterpriseRepository is a mock implementation of EnterpriseRepository
type MockEnterpriseRepository struct {
	mock.Mock
}

func (m *MockEnterpriseRepository) Create(ctx context.Context, enterprise *models.Enterprise) error {
	args := m.Called(ctx, enterprise)
	return args.Error(0)
}

func (m *MockEnterpriseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Enterprise, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enterprise), args.Error(1)
}

func (m *MockEnterpriseRepository) GetBySlug(ctx context.Context, slug string) (*models.Enterprise, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enterprise), args.Error(1)
}

func (m *MockEnterpriseRepository) Update(ctx context.Context, enterprise *models.Enterprise) error {
	args := m.Called(ctx, enterprise)
	return args.Error(0)
}

func (m *MockEnterpriseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEnterpriseRepository) List(ctx context.Context, limit, offset int) ([]*models.Enterprise, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Enterprise), args.Int(1), args.Error(2)
}

func TestService_CreateDevice(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceID := "android-device-123"
	serialNumber := "ABC123DEF456"

	t.Run("success", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(nil)
		
		svc := NewService(deviceRepo, enterpriseRepo, "test-project", "test-account.json")
		device, err := svc.CreateDevice(ctx, enterpriseID, deviceID, serialNumber)
		
		require.NoError(t, err)
		assert.NotNil(t, device)
		assert.Equal(t, enterpriseID, device.EnterpriseID)
		assert.Equal(t, deviceID, device.DeviceID)
		assert.Equal(t, serialNumber, device.SerialNumber)
		assert.Equal(t, models.PlatformAndroid, device.Platform)
		assert.Equal(t, models.DeviceStatusPending, device.Status)
		
		deviceRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(assert.AnError)
		
		svc := NewService(deviceRepo, enterpriseRepo, "test-project", "test-account.json")
		device, err := svc.CreateDevice(ctx, enterpriseID, deviceID, serialNumber)
		
		require.Error(t, err)
		assert.Nil(t, device)
		assert.Contains(t, err.Error(), "failed to create device")
		
		deviceRepo.AssertExpectations(t)
	})
}

func TestGenerateSimpleQRCode(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		token := "test-enrollment-token-12345"
		
		qr, err := GenerateSimpleQRCode(token)
		require.NoError(t, err)
		assert.NotNil(t, qr)
		assert.Greater(t, len(qr), 0)
		
		// QR code should be PNG format
		assert.Equal(t, byte(0x89), qr[0]) // PNG magic number
		assert.Equal(t, byte('P'), qr[1])
		assert.Equal(t, byte('N'), qr[2])
		assert.Equal(t, byte('G'), qr[3])
	})

	t.Run("empty token", func(t *testing.T) {
		qr, err := GenerateSimpleQRCode("")
		require.Error(t, err)
		assert.Nil(t, qr)
		// Empty token should fail
	})

	t.Run("long token", func(t *testing.T) {
		// Test with a very long token
		token := string(make([]byte, 1000))
		for i := range token {
			token = token[:i] + "a"
		}
		
		qr, err := GenerateSimpleQRCode(token)
		require.NoError(t, err)
		assert.NotNil(t, qr)
	})
}

func TestGenerateQRCode(t *testing.T) {
	token := "test-token"
	downloadURL := "https://play.google.com/managed/downloadManagingApp"
	wifiSSID := "TestWiFi"
	wifiPassword := "password123"

	t.Run("success with WiFi", func(t *testing.T) {
		qr, err := GenerateQRCode(token, downloadURL, wifiSSID, wifiPassword)
		require.NoError(t, err)
		assert.NotNil(t, qr)
		assert.Greater(t, len(qr), 0)
	})

	t.Run("success without WiFi", func(t *testing.T) {
		qr, err := GenerateQRCode(token, downloadURL, "", "")
		require.NoError(t, err)
		assert.NotNil(t, qr)
		assert.Greater(t, len(qr), 0)
	})

	t.Run("empty parameters", func(t *testing.T) {
		qr, err := GenerateQRCode("", "", "", "")
		require.NoError(t, err)
		assert.NotNil(t, qr)
	})
}

func BenchmarkGenerateSimpleQRCode(b *testing.B) {
	token := "test-enrollment-token-12345"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateSimpleQRCode(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateQRCode(b *testing.B) {
	token := "test-token"
	downloadURL := "https://play.google.com/managed/downloadManagingApp"
	wifiSSID := "TestWiFi"
	wifiPassword := "password123"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateQRCode(token, downloadURL, wifiSSID, wifiPassword)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- Service UpdateDeviceStatus Tests ---

func TestService_UpdateDeviceStatus(t *testing.T) {
	ctx := context.Background()
	deviceID := uuid.New()

	t.Run("success", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		existing := &models.Device{
			BaseModel: models.BaseModel{ID: deviceID},
			Platform:  models.PlatformAndroid,
			Status:    models.DeviceStatusPending,
		}
		deviceRepo.On("GetByID", ctx, deviceID).Return(existing, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

		svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)

		require.NoError(t, err)
		assert.Equal(t, models.DeviceStatusEnrolled, existing.Status)
		deviceRepo.AssertExpectations(t)
	})

	t.Run("device not found", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		deviceRepo.On("GetByID", ctx, deviceID).Return(nil, assert.AnError)

		svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get device")
	})

	t.Run("update error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		existing := &models.Device{
			BaseModel: models.BaseModel{ID: deviceID},
			Status:    models.DeviceStatusPending,
		}
		deviceRepo.On("GetByID", ctx, deviceID).Return(existing, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(assert.AnError)

		svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update device")
	})
}

// --- Webhook Handler Tests ---

func TestWebhookHandler_HandleWebhook(t *testing.T) {
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	// nil client — we test dispatch logic, not Google API calls
	handler := NewWebhookHandler(svc, nil, logger)

	t.Run("handles COMPLIANCE_REPORT", func(t *testing.T) {
		event := WebhookEvent{
			NotificationType: "COMPLIANCE_REPORT",
			DeviceName:       "enterprises/test/devices/123",
			Timestamp:        "2026-04-17T00:00:00Z",
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles UNENROLLMENT", func(t *testing.T) {
		event := WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "enterprises/test/devices/123",
			Timestamp:        "2026-04-17T00:00:00Z",
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles unknown notification type", func(t *testing.T) {
		event := WebhookEvent{
			NotificationType: "UNKNOWN_TYPE",
			Timestamp:        "2026-04-17T00:00:00Z",
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("handles ENROLLMENT with missing device name", func(t *testing.T) {
		event := WebhookEvent{
			NotificationType: "ENROLLMENT",
			DeviceName:       "",
			Timestamp:        "2026-04-17T00:00:00Z",
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		// Should return 500 because enrollment handler returns error for empty device name
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
