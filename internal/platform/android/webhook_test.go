package android

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/androidmanagement/v1"
	"google.golang.org/api/option"
)

func newTestWebhookHandler(deviceRepo *MockDeviceRepository, enterpriseRepo *MockEnterpriseRepository) *WebhookHandler {
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	return NewWebhookHandler(svc, nil, logger)
}

func TestHandleEnrollment_WithEnterpriseToken(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()

	t.Run("success - creates device", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: enterpriseID},
			Slug:      "test-enterprise",
		}
		enterpriseRepo.On("GetBySlug", ctx, "test-enterprise").Return(enterprise, nil)
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "ENROLLMENT",
			DeviceName:       "enterprises/test/devices/abc",
			EnterpriseToken:  "test-enterprise",
		}

		err := handler.handleEnrollment(ctx, event)
		require.NoError(t, err)
		enterpriseRepo.AssertExpectations(t)
		deviceRepo.AssertExpectations(t)
	})

	t.Run("enterprise not found - returns nil", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterpriseRepo.On("GetBySlug", ctx, "unknown").Return(nil, fmt.Errorf("not found"))

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "ENROLLMENT",
			DeviceName:       "enterprises/test/devices/abc",
			EnterpriseToken:  "unknown",
		}

		err := handler.handleEnrollment(ctx, event)
		require.NoError(t, err) // logs warning, returns nil
		enterpriseRepo.AssertExpectations(t)
	})

	t.Run("create device fails", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: enterpriseID},
			Slug:      "test-enterprise",
		}
		enterpriseRepo.On("GetBySlug", ctx, "test-enterprise").Return(enterprise, nil)
		deviceRepo.On("Create", ctx, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db error"))

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "ENROLLMENT",
			DeviceName:       "enterprises/test/devices/abc",
			EnterpriseToken:  "test-enterprise",
		}

		err := handler.handleEnrollment(ctx, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create device")
	})

	t.Run("no enterprise token - skips creation", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "ENROLLMENT",
			DeviceName:       "enterprises/test/devices/abc",
			EnterpriseToken:  "",
		}

		err := handler.handleEnrollment(ctx, event)
		require.NoError(t, err)
		// No repo calls expected
		deviceRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
}

func TestHandleUnenrollment_FullPath(t *testing.T) {
	ctx := context.Background()
	enterpriseID := uuid.New()
	deviceID := uuid.New()

	t.Run("success - updates device status", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: enterpriseID},
		}
		device := &models.Device{
			BaseModel: models.BaseModel{ID: deviceID},
			Status:    models.DeviceStatusEnrolled,
		}

		enterpriseRepo.On("GetBySlug", ctx, "ent-token").Return(enterprise, nil)
		deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/test/devices/xyz").Return(device, nil)
		deviceRepo.On("GetByID", ctx, deviceID).Return(device, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(nil)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "enterprises/test/devices/xyz",
			EnterpriseToken:  "ent-token",
		}

		err := handler.handleUnenrollment(ctx, event)
		require.NoError(t, err)
		enterpriseRepo.AssertExpectations(t)
		deviceRepo.AssertExpectations(t)
	})

	t.Run("enterprise not found - returns nil", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterpriseRepo.On("GetBySlug", ctx, "bad-token").Return(nil, fmt.Errorf("not found"))

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "enterprises/test/devices/xyz",
			EnterpriseToken:  "bad-token",
		}

		err := handler.handleUnenrollment(ctx, event)
		require.NoError(t, err)
	})

	t.Run("device not found - returns nil", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: enterpriseID},
		}
		enterpriseRepo.On("GetBySlug", ctx, "ent-token").Return(enterprise, nil)
		deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/test/devices/xyz").Return(nil, fmt.Errorf("not found"))

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "enterprises/test/devices/xyz",
			EnterpriseToken:  "ent-token",
		}

		err := handler.handleUnenrollment(ctx, event)
		require.NoError(t, err)
	})

	t.Run("update fails - returns error", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		enterprise := &models.Enterprise{
			BaseModel: models.BaseModel{ID: enterpriseID},
		}
		device := &models.Device{
			BaseModel: models.BaseModel{ID: deviceID},
			Status:    models.DeviceStatusEnrolled,
		}

		enterpriseRepo.On("GetBySlug", ctx, "ent-token").Return(enterprise, nil)
		deviceRepo.On("GetBySerial", ctx, enterpriseID, "enterprises/test/devices/xyz").Return(device, nil)
		deviceRepo.On("GetByID", ctx, deviceID).Return(device, nil)
		deviceRepo.On("Update", ctx, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db error"))

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "enterprises/test/devices/xyz",
			EnterpriseToken:  "ent-token",
		}

		err := handler.handleUnenrollment(ctx, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update device status")
	})

	t.Run("empty device name - skips", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "UNENROLLMENT",
			DeviceName:       "",
			EnterpriseToken:  "ent-token",
		}

		err := handler.handleUnenrollment(ctx, event)
		require.NoError(t, err)
	})
}

func TestHandleStatusReport(t *testing.T) {
	ctx := context.Background()

	t.Run("empty device name - returns nil", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "STATUS_REPORT",
			DeviceName:       "",
		}

		err := handler.handleStatusReport(ctx, event)
		require.NoError(t, err)
	})

	t.Run("client nil - panics avoided by empty device name check", func(t *testing.T) {
		// With a non-empty device name and nil client, it will panic.
		// This tests the empty-name early return path.
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)

		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)
		event := &WebhookEvent{
			NotificationType: "STATUS_REPORT",
			DeviceName:       "",
		}

		err := handler.handleStatusReport(ctx, event)
		require.NoError(t, err)
	})
}

func TestHandleWebhook_StatusReport(t *testing.T) {
	t.Run("STATUS_REPORT with empty device name returns 200", func(t *testing.T) {
		deviceRepo := new(MockDeviceRepository)
		enterpriseRepo := new(MockEnterpriseRepository)
		handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)

		event := WebhookEvent{
			NotificationType: "STATUS_REPORT",
			DeviceName:       "",
			Timestamp:        "2026-04-17T00:00:00Z",
		}
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.HandleWebhook(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleWebhook_EnrollmentSuccess(t *testing.T) {
	enterpriseID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: enterpriseID},
	}
	enterpriseRepo.On("GetBySlug", mock.Anything, "my-ent").Return(enterprise, nil)
	deviceRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Device")).Return(nil)

	handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)

	event := WebhookEvent{
		NotificationType: "ENROLLMENT",
		DeviceName:       "enterprises/test/devices/new",
		EnterpriseToken:  "my-ent",
		Timestamp:        "2026-04-17T00:00:00Z",
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleWebhook(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	deviceRepo.AssertExpectations(t)
}

func TestNewPoller(t *testing.T) {
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))

	poller := NewPoller(svc, nil, logger)
	require.NotNil(t, poller)
	assert.Equal(t, svc, poller.service)
	assert.Nil(t, poller.client)
	assert.Equal(t, logger, poller.logger)
}

func newMockClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc, err := androidmanagement.NewService(context.Background(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL),
	)
	require.NoError(t, err)

	return &Client{
		service:   svc,
		projectID: "test-project",
		logger:    slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

func TestPoll_Success(t *testing.T) {
	// Mock server returns empty device list
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"devices": []interface{}{
				map[string]interface{}{
					"name":  "enterprises/test/devices/d1",
					"state": "ACTIVE",
				},
			},
		})
	})

	client := newMockClient(t, mockHandler)
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))

	poller := NewPoller(svc, client, logger)
	err := poller.Poll(context.Background(), "enterprises/test")
	require.NoError(t, err)
}

func TestPoll_ListError(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})

	client := newMockClient(t, mockHandler)
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))

	poller := NewPoller(svc, client, logger)
	err := poller.Poll(context.Background(), "enterprises/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list devices")
}

func TestHandleStatusReport_WithClient(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":         "enterprises/test/devices/d1",
			"state":        "ACTIVE",
			"appliedState": "ACTIVE",
		})
	})

	client := newMockClient(t, mockHandler)
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))

	handler := &WebhookHandler{service: svc, client: client, logger: logger}
	event := &WebhookEvent{
		NotificationType: "STATUS_REPORT",
		DeviceName:       "enterprises/test/devices/d1",
	}

	err := handler.handleStatusReport(context.Background(), event)
	require.NoError(t, err)
}

func TestHandleStatusReport_ClientError(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	client := newMockClient(t, mockHandler)
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	svc := NewService(deviceRepo, enterpriseRepo, "proj", "sa.json")
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))

	handler := &WebhookHandler{service: svc, client: client, logger: logger}
	event := &WebhookEvent{
		NotificationType: "STATUS_REPORT",
		DeviceName:       "enterprises/test/devices/d1",
	}

	err := handler.handleStatusReport(context.Background(), event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get device")
}

func TestHandleWebhook_UnenrollmentError(t *testing.T) {
	enterpriseID := uuid.New()
	deviceID := uuid.New()
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)

	enterprise := &models.Enterprise{
		BaseModel: models.BaseModel{ID: enterpriseID},
	}
	device := &models.Device{
		BaseModel: models.BaseModel{ID: deviceID},
		Status:    models.DeviceStatusEnrolled,
	}

	enterpriseRepo.On("GetBySlug", mock.Anything, "ent").Return(enterprise, nil)
	deviceRepo.On("GetBySerial", mock.Anything, enterpriseID, "enterprises/x/devices/1").Return(device, nil)
	deviceRepo.On("GetByID", mock.Anything, deviceID).Return(device, nil)
	deviceRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Device")).Return(fmt.Errorf("db fail"))

	handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)

	event := WebhookEvent{
		NotificationType: "UNENROLLMENT",
		DeviceName:       "enterprises/x/devices/1",
		EnterpriseToken:  "ent",
		Timestamp:        "2026-04-17T00:00:00Z",
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleWebhook(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleWebhook_ComplianceReportWithData(t *testing.T) {
	deviceRepo := new(MockDeviceRepository)
	enterpriseRepo := new(MockEnterpriseRepository)
	handler := newTestWebhookHandler(deviceRepo, enterpriseRepo)

	event := WebhookEvent{
		NotificationType: "COMPLIANCE_REPORT",
		DeviceName:       "enterprises/test/devices/456",
		Timestamp:        "2026-04-17T00:00:00Z",
		Data: map[string]interface{}{
			"nonComplianceDetails": []interface{}{"policy_violation"},
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleWebhook(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
