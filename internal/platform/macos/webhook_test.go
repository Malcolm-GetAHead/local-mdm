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
