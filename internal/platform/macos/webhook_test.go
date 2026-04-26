package macos

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestCheckinHandler_Authenticate(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := WebhookEvent{
		Topic:   "mdm.Authenticate",
		EventID: "evt-1",
		CheckinEvent: &CheckinEvent{
			UDID:        "AAAA-BBBB-CCCC",
			MessageType: "Authenticate",
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckinHandler_TokenUpdate(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := WebhookEvent{
		Topic: "mdm.TokenUpdate",
		CheckinEvent: &CheckinEvent{
			UDID:        "AAAA-BBBB-CCCC",
			MessageType: "TokenUpdate",
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckinHandler_InvalidJSON(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckinHandler_NoCheckinEvent(t *testing.T) {
	h := NewCheckinHandler(testNanoMDMService(t), testMacOSService(t), nil,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := WebhookEvent{Topic: "mdm.Connect"}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/checkin", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCommandHandler_CommandResult(t *testing.T) {
	h := NewCommandHandler(testNanoMDMService(t),
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := CommandWebhookEvent{
		Topic: "mdm.Connect",
		CommandEvent: &CommandEvent{
			UDID:        "AAAA-BBBB-CCCC",
			CommandUUID: "cmd-123",
			Status:      "Acknowledged",
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/mdm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCommandHandler_InvalidJSON(t *testing.T) {
	h := NewCommandHandler(testNanoMDMService(t),
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	req := httptest.NewRequest("PUT", "/mdm", bytes.NewReader([]byte("{bad")))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCommandHandler_NoCommandEvent(t *testing.T) {
	h := NewCommandHandler(testNanoMDMService(t),
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	event := CommandWebhookEvent{Topic: "mdm.Connect"}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("PUT", "/mdm", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
