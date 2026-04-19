package macos

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"log/slog"
	"math/big"
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

func TestService_CreateDevice(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	udid := "test-udid-12345"
	serialNumber := "C02ABC123DEF"

	t.Run("success", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(nil)
		
		svc := NewService(deviceRepo)
		device, err := svc.CreateDevice(ctx, enterpriseID, udid, serialNumber)
		
		require.NoError(t, err)
		assert.NotNil(t, device)
		assert.Equal(t, enterpriseID, device.EnterpriseID)
		assert.Equal(t, udid, device.DeviceID)
		assert.Equal(t, serialNumber, device.SerialNumber)
		assert.Equal(t, models.PlatformMacOS, device.Platform)
		assert.Equal(t, models.DeviceStatusPending, device.Status)
		
		deviceRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(assert.AnError)
		
		svc := NewService(deviceRepo)
		device, err := svc.CreateDevice(ctx, enterpriseID, udid, serialNumber)
		
		require.Error(t, err)
		assert.Nil(t, device)
		assert.Contains(t, err.Error(), "failed to create device")
		
		deviceRepo.AssertExpectations(t)
	})
}

func TestService_UpdateDeviceStatus(t *testing.T) {
	ctx := context.Background()
	deviceID := uuid.New()
	enterpriseID := uuid.New()

	t.Run("success", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		existingDevice := &models.Device{
			BaseModel:    models.BaseModel{ID: deviceID},
			EnterpriseID: enterpriseID,
			Platform:     models.PlatformMacOS,
			Status:       models.DeviceStatusPending,
		}
		
		deviceRepo.On("GetByID", ctx, deviceID).Return(existingDevice, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)
		
		svc := NewService(deviceRepo)
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)
		
		require.NoError(t, err)
		assert.Equal(t, models.DeviceStatusEnrolled, existingDevice.Status)
		
		deviceRepo.AssertExpectations(t)
	})

	t.Run("device not found", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		deviceRepo.On("GetByID", ctx, deviceID).Return(nil, assert.AnError)
		
		svc := NewService(deviceRepo)
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get device")
		
		deviceRepo.AssertExpectations(t)
	})

	t.Run("update error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		
		existingDevice := &models.Device{
			BaseModel:    models.BaseModel{ID: deviceID},
			EnterpriseID: enterpriseID,
			Platform:     models.PlatformMacOS,
			Status:       models.DeviceStatusPending,
		}
		
		deviceRepo.On("GetByID", ctx, deviceID).Return(existingDevice, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(assert.AnError)
		
		svc := NewService(deviceRepo)
		err := svc.UpdateDeviceStatus(ctx, deviceID, models.DeviceStatusEnrolled)
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update device")
		
		deviceRepo.AssertExpectations(t)
	})
}

func TestGenerateEnrollmentProfile(t *testing.T) {
	enterpriseID := uuid.New()
	serverURL := "https://mdm.example.com"
	scepURL := "https://mdm.example.com/scep"
	topic := "com.example.mdm"
	challenge := "test-challenge"
	orgName := "Test Organization"

	t.Run("success", func(t *testing.T) {
		profile, err := GenerateEnrollmentProfile(
			enterpriseID,
			serverURL,
			scepURL,
			topic,
			challenge,
			orgName,
			nil,
		)
		
		require.NoError(t, err)
		assert.NotNil(t, profile)
		assert.Greater(t, len(profile), 0)
		
		// Verify profile contains expected elements
		profileStr := string(profile)
		assert.Contains(t, profileStr, "<?xml version=\"1.0\"")
		assert.Contains(t, profileStr, "<!DOCTYPE plist")
		assert.Contains(t, profileStr, serverURL)
		assert.Contains(t, profileStr, scepURL)
		assert.Contains(t, profileStr, topic)
		assert.Contains(t, profileStr, challenge)
		assert.Contains(t, profileStr, orgName)
		assert.Contains(t, profileStr, "com.apple.mdm")
		assert.Contains(t, profileStr, "com.apple.security.scep")
	})

	t.Run("empty parameters", func(t *testing.T) {
		profile, err := GenerateEnrollmentProfile(
			uuid.Nil,
			"",
			"",
			"",
			"",
			"",
			nil,
		)
		
		// Should still generate a profile, just with empty values
		require.NoError(t, err)
		assert.NotNil(t, profile)
	})
}

func BenchmarkGenerateEnrollmentProfile(b *testing.B) {
	enterpriseID := uuid.New()
	serverURL := "https://mdm.example.com"
	scepURL := "https://mdm.example.com/scep"
	topic := "com.example.mdm"
	challenge := "test-challenge"
	orgName := "Test Organization"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateEnrollmentProfile(
			enterpriseID,
			serverURL,
			scepURL,
			topic,
			challenge,
			orgName,
			nil,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- Webhook Handler Tests ---

func TestWebhookHandler_HandleWebhook(t *testing.T) {
	deviceRepo := new(MockDeviceRepository)
	svc := NewService(deviceRepo)
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewWebhookHandler(svc, logger)

	t.Run("handles Authenticate event", func(t *testing.T) {
		event := WebhookEvent{
			Topic:   "mdm",
			EventID: "evt-1",
			CheckinEvent: &CheckinEvent{
				UDID:        "test-udid",
				MessageType: "Authenticate",
			},
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles TokenUpdate event", func(t *testing.T) {
		event := WebhookEvent{
			Topic:   "mdm",
			EventID: "evt-2",
			CheckinEvent: &CheckinEvent{
				UDID:        "test-udid",
				MessageType: "TokenUpdate",
			},
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles CheckOut event", func(t *testing.T) {
		event := WebhookEvent{
			Topic:   "mdm",
			EventID: "evt-3",
			CheckinEvent: &CheckinEvent{
				UDID:        "test-udid",
				MessageType: "CheckOut",
			},
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles nil checkin event", func(t *testing.T) {
		event := WebhookEvent{Topic: "mdm", EventID: "evt-4"}
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
}

// --- NanoMDM Service Tests ---

func TestNanoMDMService(t *testing.T) {
	t.Run("HandleCommand logs and returns nil", func(t *testing.T) {
		svc := NewNanoMDMService("", "", nil, nil, slog.Default())

		err := svc.HandleCommand(context.Background(), "test-udid", "cmd-1", "Acknowledged")
		assert.NoError(t, err)
	})

	t.Run("HandleCheckin logs and returns nil", func(t *testing.T) {
		svc := NewNanoMDMService("", "", nil, nil, slog.Default())

		err := svc.HandleCheckin(context.Background(), "test-udid", "Authenticate")
		assert.NoError(t, err)
	})

	t.Run("SendCommand returns nil when not configured", func(t *testing.T) {
		svc := NewNanoMDMService("", "", nil, nil, slog.Default())

		resp, err := svc.SendCommand(context.Background(), "test-udid", []byte("test"))
		assert.NoError(t, err)
		assert.Nil(t, resp)
	})
}

// --- CheckinHandler and CommandHandler Tests ---

func TestCheckinHandler_ServeHTTP(t *testing.T) {
	svc := NewNanoMDMService("", "", nil, nil, slog.Default())
	deviceRepo := new(MockDeviceRepository)
	service := NewService(deviceRepo)
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewCheckinHandler(svc, service, logger)

	event := WebhookEvent{Topic: "mdm.Authenticate", CheckinEvent: &CheckinEvent{UDID: "test", MessageType: "Authenticate"}}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCommandHandler_ServeHTTP(t *testing.T) {
	svc := NewNanoMDMService("", "", nil, nil, slog.Default())
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewCommandHandler(svc, logger)

	event := CommandWebhookEvent{Topic: "mdm.Connect", CommandEvent: &CommandEvent{UDID: "test", CommandUUID: "cmd-1", Status: "Acknowledged"}}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/mdm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Enrollment Profile with CA Cert ---

func TestGenerateEnrollmentProfile_WithCACert(t *testing.T) {
	// Generate a self-signed CA cert for testing
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	profile, err := GenerateEnrollmentProfile(
		uuid.New(), "https://mdm.example.com", "https://mdm.example.com/scep",
		"com.example.mdm", "challenge", "Test Org", caCert,
	)
	require.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Contains(t, string(profile), "com.apple.mdm")
}
