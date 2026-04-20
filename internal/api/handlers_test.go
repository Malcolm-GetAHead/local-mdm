package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleListEnterprises(t *testing.T) {
	t.Run("returns empty list", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/enterprises", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.NotNil(t, resp.Data)
		assert.NotNil(t, resp.Meta)
		assert.Equal(t, 0, resp.Meta.Total)
	})

	t.Run("returns enterprises with pagination", func(t *testing.T) {
		ts := newTestServer(t)
		for i := 0; i < 3; i++ {
			ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
				BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				Name:      fmt.Sprintf("Enterprise %d", i),
				Slug:      fmt.Sprintf("ent-%d", i),
			})
		}

		req := httptest.NewRequest("GET", "/api/v1/enterprises?limit=2&offset=0", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 3, resp.Meta.Total)
		assert.Equal(t, 2, resp.Meta.PerPage)
		assert.Equal(t, 1, resp.Meta.Page)
	})

	t.Run("handles repo error", func(t *testing.T) {
		ts := newTestServer(t)
		ts.enterpriseRepo.listErr = fmt.Errorf("db connection lost")

		req := httptest.NewRequest("GET", "/api/v1/enterprises", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleCreateEnterprise(t *testing.T) {
	t.Run("creates enterprise successfully", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "Acme Corp", "slug": "acme"})
		req := httptest.NewRequest("POST", "/api/v1/enterprises", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, ts.enterpriseRepo.enterprises, 1)
		assert.Equal(t, "Acme Corp", ts.enterpriseRepo.enterprises[0].Name)
		assert.Equal(t, "acme", ts.enterpriseRepo.enterprises[0].Slug)
		// Verify audit log was created
		assert.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "enterprise.create", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "", "slug": "test"})
		req := httptest.NewRequest("POST", "/api/v1/enterprises", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "validation_failed", resp.Error.Code)
	})

	t.Run("rejects empty slug", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "Test", "slug": ""})
		req := httptest.NewRequest("POST", "/api/v1/enterprises", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/enterprises", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("handles duplicate slug", func(t *testing.T) {
		ts := newTestServer(t)
		ts.enterpriseRepo.createErr = fmt.Errorf("duplicate key value violates unique constraint")

		body := jsonBody(t, map[string]string{"name": "Test", "slug": "existing"})
		req := httptest.NewRequest("POST", "/api/v1/enterprises", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestHandleGetEnterprise(t *testing.T) {
	t.Run("returns enterprise by ID", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "Test Corp",
			Slug:      "test",
		})

		req := httptest.NewRequest("GET", "/api/v1/enterprises/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/enterprises/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "not_found", resp.Error.Code)
	})

	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/enterprises/not-a-uuid", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "invalid_id", resp.Error.Code)
	})

	t.Run("handles repo error", func(t *testing.T) {
		ts := newTestServer(t)
		ts.enterpriseRepo.getErr = fmt.Errorf("connection timeout")

		req := httptest.NewRequest("GET", "/api/v1/enterprises/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleListDevices(t *testing.T) {
	t.Run("returns devices scoped to enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Platform:  models.PlatformWindows,
			Status:    models.DeviceStatusEnrolled,
		})

		req := httptest.NewRequest("GET", "/api/v1/devices", nil)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("returns 401 without auth context", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("respects pagination params", func(t *testing.T) {
		ts := newTestServer(t)
		for i := 0; i < 5; i++ {
			ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
				BaseModel: models.BaseModel{ID: uuid.New()},
			})
		}

		req := httptest.NewRequest("GET", "/api/v1/devices?limit=2&offset=2", nil)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 5, resp.Meta.Total)
		assert.Equal(t, 2, resp.Meta.PerPage)
		assert.Equal(t, 2, resp.Meta.Page)
	})
}

func TestHandleGetDevice(t *testing.T) {
	t.Run("returns device by ID", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformMacOS,
			Name:      "Test MacBook",
		})

		req := httptest.NewRequest("GET", "/api/v1/devices/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/bad-id", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleLockDevice(t *testing.T) {
	t.Run("locks device and audit logs", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformWindows,
			Status:    models.DeviceStatusEnrolled,
		})

		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/lock", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, models.DeviceStatusLost, ts.deviceRepo.devices[0].Status)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "device.lock", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/lock", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("handles update error", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Status:    models.DeviceStatusEnrolled,
		})
		ts.deviceRepo.updateErr = fmt.Errorf("db write failed")

		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/lock", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleWipeDevice(t *testing.T) {
	t.Run("wipes device and audit logs", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformAndroid,
			Status:    models.DeviceStatusEnrolled,
		})

		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/wipe", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, models.DeviceStatusWiped, ts.deviceRepo.devices[0].Status)
		require.Len(t, ts.auditLogger.events, 2)
		assert.Equal(t, "device.wipe", ts.auditLogger.events[0].Action)
		assert.Equal(t, "device.lifecycle.wipe", ts.auditLogger.events[1].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/wipe", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleListPolicies(t *testing.T) {
	t.Run("returns policies scoped to enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "WiFi Policy",
			Platform:  models.PlatformMacOS,
		})

		req := httptest.NewRequest("GET", "/api/v1/policies", nil)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleCreatePolicy(t *testing.T) {
	t.Run("creates policy successfully", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name":          "Security Policy",
			"platform":      "windows",
			"policy_type":   "security",
			"policy_config": map[string]interface{}{"firewall": true},
			"is_active":     true,
		})
		req := httptest.NewRequest("POST", "/api/v1/policies", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.policyRepo.policies, 1)
		assert.Equal(t, "Security Policy", ts.policyRepo.policies[0].Name)
		assert.Equal(t, testUser().EnterpriseID, ts.policyRepo.policies[0].EnterpriseID)
		// Verify audit
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "policy.create", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name": "", "platform": "windows", "policy_type": "security",
		})
		req := httptest.NewRequest("POST", "/api/v1/policies", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects invalid platform", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name": "Test", "platform": "linux", "policy_type": "security",
		})
		req := httptest.NewRequest("POST", "/api/v1/policies", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusBadRequest, w.Code)
		resp := decodeResponse(t, w)
		assert.Contains(t, resp.Error.Message, "platform")
	})

	t.Run("rejects invalid policy type", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name": "Test", "platform": "windows", "policy_type": "invalid",
		})
		req := httptest.NewRequest("POST", "/api/v1/policies", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name": "Test", "platform": "windows", "policy_type": "security",
		})
		req := httptest.NewRequest("POST", "/api/v1/policies", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleGetPolicy(t *testing.T) {
	t.Run("returns policy by ID", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "Test Policy",
		})

		req := httptest.NewRequest("GET", "/api/v1/policies/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing policy", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/not-valid", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleListCertificates(t *testing.T) {
	t.Run("returns certificates", func(t *testing.T) {
		ts := newTestServer(t)
		ts.certRepo.certs = append(ts.certRepo.certs, &models.Certificate{
			BaseModel:    models.BaseModel{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			CertType:     "device",
			SerialNumber: "123456",
		})

		req := httptest.NewRequest("GET", "/api/v1/certificates", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("accepts device_id filter", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/certificates?device_id="+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects invalid device_id", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/certificates?device_id=bad", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleListAuditLogs(t *testing.T) {
	t.Run("returns audit logs scoped to enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		entID := testUser().EnterpriseID
		ts.auditLogRepo.logs = append(ts.auditLogRepo.logs, &models.AuditLog{
			ID:           uuid.New(),
			EnterpriseID: &entID,
			Action:       "device.create",
			ResourceType: "device",
			CreatedAt:    time.Now(),
		})

		req := httptest.NewRequest("GET", "/api/v1/audit-logs", nil)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/audit-logs", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleAndroidWebhook(t *testing.T) {
	t.Run("accepts request without secret configured", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/android/webhook", jsonBody(t, map[string]string{"type": "test"}))
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects missing signature when secret configured", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.config.Android.WebhookSecret = "test-secret"

		req := httptest.NewRequest("POST", "/api/v1/android/webhook", jsonBody(t, map[string]string{"type": "test"}))
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects invalid signature", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.config.Android.WebhookSecret = "test-secret"

		req := httptest.NewRequest("POST", "/api/v1/android/webhook", jsonBody(t, map[string]string{"type": "test"}))
		req.Header.Set("X-Webhook-Signature", "invalid-sig")
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("accepts valid HMAC signature", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.config.Android.WebhookSecret = "test-secret"

		payload := []byte(`{"type":"test"}`)
		sig := computeHMAC(payload, []byte("test-secret"))

		req := httptest.NewRequest("POST", "/api/v1/android/webhook", bytes.NewReader(payload))
		req.Header.Set("X-Webhook-Signature", sig)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleMacOSEnrollmentProfile(t *testing.T) {
	t.Run("rejects invalid enterprise ID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/macos/enroll/not-a-uuid", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/macos/enroll/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("generates profile for valid enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id},
			Name:      "Test",
			Slug:      "test",
		})

		req := httptest.NewRequest("GET", "/api/v1/macos/enroll/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-apple-aspen-config", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "enrollment.mobileconfig")
		// Verify audit log
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "enrollment.macos.profile_generated", ts.auditLogger.events[0].Action)
	})
}

// Helper for HMAC tests
func computeHMAC(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// --- DEP Handler Tests ---

func TestHandleDEPTokenPKI(t *testing.T) {
	t.Run("generates token PKI certificate", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/dep/test-server/tokenpki", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/x-pem-file", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "BEGIN CERTIFICATE")
	})
}

func TestHandleDEPAssignerProfile(t *testing.T) {
	t.Run("get empty profile", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/dep/test-server/assigner", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("set profile", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"profile_uuid": "uuid-123"})
		req := httptest.NewRequest("PUT", "/api/v1/dep/test-server/assigner", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects empty profile UUID", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"profile_uuid": ""})
		req := httptest.NewRequest("PUT", "/api/v1/dep/test-server/assigner", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleDEPDevices(t *testing.T) {
	t.Run("returns empty device list", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/dep/test-server/devices", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 0, resp.Meta.Total)
	})
}

// --- Duplicate Prevention Tests (M-05) ---

func TestIsDuplicateError(t *testing.T) {
	assert.True(t, isDuplicateError(fmt.Errorf("duplicate key value violates unique constraint")))
	assert.True(t, isDuplicateError(fmt.Errorf("unique_violation")))
	assert.True(t, isDuplicateError(fmt.Errorf("resource already exists")))
	assert.False(t, isDuplicateError(fmt.Errorf("connection refused")))
	assert.False(t, isDuplicateError(fmt.Errorf("not found")))
}

// --- S2a-01: New CRUD Endpoint Tests ---

func TestHandleUpdateEnterprise(t *testing.T) {
	t.Run("updates enterprise name", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "Old Name",
			Slug:      "old",
		})

		body := jsonBody(t, map[string]string{"name": "New Name"})
		req := httptest.NewRequest("PUT", "/api/v1/enterprises/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "New Name", ts.enterpriseRepo.enterprises[0].Name)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "enterprise.update", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "New"})
		req := httptest.NewRequest("PUT", "/api/v1/enterprises/"+uuid.New().String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id},
			Name:      "Existing",
		})

		name := ""
		body := jsonBody(t, map[string]*string{"name": &name})
		req := httptest.NewRequest("PUT", "/api/v1/enterprises/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("PUT", "/api/v1/enterprises/bad-id", jsonBody(t, map[string]string{"name": "X"}))
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleDeleteEnterprise(t *testing.T) {
	t.Run("deletes enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id},
			Name:      "To Delete",
		})

		req := httptest.NewRequest("DELETE", "/api/v1/enterprises/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Len(t, ts.enterpriseRepo.enterprises, 0)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "enterprise.delete", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/enterprises/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUpdateDevice(t *testing.T) {
	t.Run("updates device fields", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformWindows,
			Name:      "Old",
			Status:    models.DeviceStatusEnrolled,
		})

		body := jsonBody(t, map[string]string{"name": "New Name", "model": "Surface Pro"})
		req := httptest.NewRequest("PUT", "/api/v1/devices/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "New Name", ts.deviceRepo.devices[0].Name)
		assert.Equal(t, "Surface Pro", ts.deviceRepo.devices[0].Model)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "device.update", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "X"})
		req := httptest.NewRequest("PUT", "/api/v1/devices/"+uuid.New().String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeleteDevice(t *testing.T) {
	t.Run("deletes device", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
		})

		req := httptest.NewRequest("DELETE", "/api/v1/devices/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Len(t, ts.deviceRepo.devices, 0)
		require.Len(t, ts.auditLogger.events, 2)
		assert.Equal(t, "device.delete", ts.auditLogger.events[0].Action)
		assert.Equal(t, "device.lifecycle.delete", ts.auditLogger.events[1].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/devices/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUpdatePolicy(t *testing.T) {
	t.Run("updates policy fields", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "Old Policy",
			IsActive:  false,
		})

		active := true
		body := jsonBody(t, map[string]interface{}{"name": "Updated Policy", "is_active": active})
		req := httptest.NewRequest("PUT", "/api/v1/policies/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "Updated Policy", ts.policyRepo.policies[0].Name)
		assert.True(t, ts.policyRepo.policies[0].IsActive)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "policy.update", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: id},
			Name:      "Existing",
		})

		name := ""
		body := jsonBody(t, map[string]*string{"name": &name})
		req := httptest.NewRequest("PUT", "/api/v1/policies/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing policy", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "X"})
		req := httptest.NewRequest("PUT", "/api/v1/policies/"+uuid.New().String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeletePolicy(t *testing.T) {
	t.Run("deletes policy", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: id},
		})

		req := httptest.NewRequest("DELETE", "/api/v1/policies/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Len(t, ts.policyRepo.policies, 0)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "policy.delete", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing policy", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/policies/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleAssignPolicy(t *testing.T) {
	t.Run("assigns policy to devices", func(t *testing.T) {
		ts := newTestServer(t)
		policyID := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: policyID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:      "Test Policy",
		})

		deviceIDs := []uuid.UUID{uuid.New(), uuid.New()}
		body := jsonBody(t, map[string]interface{}{"device_ids": deviceIDs})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+policyID.String()+"/assign", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Len(t, ts.policyRepo.assignments, 2)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "policy.assign", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects empty device_ids", func(t *testing.T) {
		ts := newTestServer(t)
		policyID := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel: models.BaseModel{ID: policyID},
		})

		body := jsonBody(t, map[string]interface{}{"device_ids": []uuid.UUID{}})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+policyID.String()+"/assign", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing policy", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"device_ids": []uuid.UUID{uuid.New()}})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+uuid.New().String()+"/assign", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUnassignPolicy(t *testing.T) {
	t.Run("unassigns policy from device", func(t *testing.T) {
		ts := newTestServer(t)
		policyID := uuid.New()
		deviceID := uuid.New()

		req := httptest.NewRequest("DELETE", "/api/v1/policies/"+policyID.String()+"/assign/"+deviceID.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "policy.unassign", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 400 for invalid policy ID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/policies/bad-id/assign/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid device ID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/policies/"+uuid.New().String()+"/assign/bad-id", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Sprint 3: Command Handlers ---

func TestHandleSendCommand(t *testing.T) {
	t.Run("sends command successfully", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformWindows,
		})

		body := jsonBody(t, map[string]interface{}{"command_type": "device_info"})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/commands", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.commandRepo.commands, 1)
		assert.Equal(t, "device_info", ts.commandRepo.commands[0].CommandType)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "command.send", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"command_type": "lock"})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/commands", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for missing command_type", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
		})

		body := jsonBody(t, map[string]interface{}{"command_type": ""})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/commands", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleListCommands(t *testing.T) {
	t.Run("lists commands for device", func(t *testing.T) {
		ts := newTestServer(t)
		deviceID := uuid.New()
		ts.commandRepo.commands = append(ts.commandRepo.commands, &models.DeviceCommand{
			BaseModel:   models.BaseModel{ID: uuid.New()},
			DeviceID:    deviceID,
			CommandType: "lock",
			Status:      "pending",
		})

		req := httptest.NewRequest("GET", "/api/v1/devices/"+deviceID.String()+"/commands", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("returns 400 for invalid device ID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/bad-id/commands", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Sprint 3: Profile Handlers ---

func TestHandleInstallProfile(t *testing.T) {
	t.Run("installs profile successfully", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformMacOS,
		})

		body := jsonBody(t, map[string]interface{}{
			"profile_type": "wifi",
			"profile_data": map[string]interface{}{"ssid": "TestNet"},
		})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/profiles", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.commandRepo.commands, 1)
		assert.Equal(t, models.CommandTypeInstallProfile, ts.commandRepo.commands[0].CommandType)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "profile.install", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"profile_type": "wifi"})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/profiles", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for missing profile_type", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
		})

		body := jsonBody(t, map[string]interface{}{"profile_type": ""})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/profiles", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleRemoveProfile(t *testing.T) {
	t.Run("removes profile successfully", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformMacOS,
		})

		req := httptest.NewRequest("DELETE", "/api/v1/devices/"+id.String()+"/profiles/com.example.wifi", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.commandRepo.commands, 1)
		assert.Equal(t, models.CommandTypeRemoveProfile, ts.commandRepo.commands[0].CommandType)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "profile.remove", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/devices/"+uuid.New().String()+"/profiles/com.example.wifi", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- Sprint 3: Restart Handler ---

func TestHandleRestartDevice(t *testing.T) {
	t.Run("restarts macOS device", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformMacOS,
			Status:    models.DeviceStatusEnrolled,
		})

		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/restart", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		require.Len(t, ts.commandRepo.commands, 1)
		assert.Equal(t, models.CommandTypeRestart, ts.commandRepo.commands[0].CommandType)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "device.restart", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects Windows device", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: id},
			Platform:  models.PlatformWindows,
		})

		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/restart", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/restart", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- Sprint 3: App Management Handlers ---

func TestHandleCreateApp(t *testing.T) {
	t.Run("creates app successfully", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name":       "Slack",
			"platform":   "macos",
			"identifier": "com.slack.Slack",
			"version":    "4.0",
		})
		req := httptest.NewRequest("POST", "/api/v1/apps", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.appRepo.apps, 1)
		assert.Equal(t, "Slack", ts.appRepo.apps[0].Name)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "app.create", ts.auditLogger.events[0].Action)
	})

	t.Run("returns 400 for missing required fields", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"name": "Slack"})
		req := httptest.NewRequest("POST", "/api/v1/apps", body)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name": "Slack", "platform": "macos", "identifier": "com.slack.Slack",
		})
		req := httptest.NewRequest("POST", "/api/v1/apps", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleGetApp(t *testing.T) {
	t.Run("returns app by ID", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel:  models.BaseModel{ID: id},
			Name:       "Slack",
			Platform:   "macos",
			Identifier: "com.slack.Slack",
		})

		req := httptest.NewRequest("GET", "/api/v1/apps/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing app", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/apps/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUpdateApp(t *testing.T) {
	t.Run("updates app successfully", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel:  models.BaseModel{ID: id},
			Name:       "Slack",
			Platform:   "macos",
			Identifier: "com.slack.Slack",
			Version:    "3.0",
		})

		body := jsonBody(t, map[string]interface{}{"version": "4.0"})
		req := httptest.NewRequest("PUT", "/api/v1/apps/"+id.String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "4.0", ts.appRepo.apps[0].Version)
	})

	t.Run("returns 404 for missing app", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"version": "4.0"})
		req := httptest.NewRequest("PUT", "/api/v1/apps/"+uuid.New().String(), body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeleteApp(t *testing.T) {
	t.Run("deletes app successfully", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel: models.BaseModel{ID: id},
			Name:      "Slack",
		})

		req := httptest.NewRequest("DELETE", "/api/v1/apps/"+id.String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Len(t, ts.appRepo.apps, 0)
	})

	t.Run("returns 404 for missing app", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/apps/"+uuid.New().String(), nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleListApps(t *testing.T) {
	t.Run("lists apps for enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel: models.BaseModel{ID: uuid.New()},
			Name:      "Slack",
			Platform:  "macos",
		})

		req := httptest.NewRequest("GET", "/api/v1/apps", nil)
		w := ts.doWithAuth(req, testUser())

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 1, resp.Meta.Total)
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/apps", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleDeployApp(t *testing.T) {
	t.Run("deploys app to devices", func(t *testing.T) {
		ts := newTestServer(t)
		appID := uuid.New()
		deviceID := uuid.New()
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel:  models.BaseModel{ID: appID},
			Name:       "Slack",
			Platform:   "macos",
			Identifier: "com.slack.Slack",
		})
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel: models.BaseModel{ID: deviceID},
			Platform:  models.PlatformMacOS,
		})

		body := jsonBody(t, map[string]interface{}{"device_ids": []string{deviceID.String()}})
		req := httptest.NewRequest("POST", "/api/v1/apps/"+appID.String()+"/deploy", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		require.Len(t, ts.commandRepo.commands, 1)
		assert.Equal(t, models.CommandTypeInstallApp, ts.commandRepo.commands[0].CommandType)
	})

	t.Run("returns 404 for missing app", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"device_ids": []string{uuid.New().String()}})
		req := httptest.NewRequest("POST", "/api/v1/apps/"+uuid.New().String()+"/deploy", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for empty device_ids", func(t *testing.T) {
		ts := newTestServer(t)
		appID := uuid.New()
		ts.appRepo.apps = append(ts.appRepo.apps, &models.App{
			BaseModel: models.BaseModel{ID: appID},
			Name:      "Slack",
		})

		body := jsonBody(t, map[string]interface{}{"device_ids": []string{}})
		req := httptest.NewRequest("POST", "/api/v1/apps/"+appID.String()+"/deploy", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Sprint 3: Windows PPKG Handlers ---

func TestHandleWindowsPPKGTemplates(t *testing.T) {
	t.Run("returns available templates", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/windows/ppkg/templates", nil)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleWindowsPPKGGenerate(t *testing.T) {
	t.Run("generates basic ppkg", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.config.Server.Host = "mdm.test.com"
		ts.server.config.Server.Port = 443

		body := jsonBody(t, map[string]interface{}{
			"name":     "test-enrollment",
			"template": "enrollment_only",
		})
		req := httptest.NewRequest("POST", "/api/v1/windows/ppkg/generate", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "test-enrollment.ppkg")
	})

	t.Run("returns 400 for enrollment_wifi without wifi config", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{
			"name":     "test",
			"template": "enrollment_wifi",
		})
		req := httptest.NewRequest("POST", "/api/v1/windows/ppkg/generate", body)
		w := ts.do(req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
