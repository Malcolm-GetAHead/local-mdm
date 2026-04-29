package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/reporting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Group Handler Tests ---

func TestHandleListGroups(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns empty list", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, 0, resp.Meta.Total)
	})
}

func TestHandleCreateGroup(t *testing.T) {
	t.Run("creates group", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "Engineering", "description": "Eng team"})
		req := httptest.NewRequest("POST", "/api/v1/groups", body)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "group.create", ts.auditLogger.events[0].Action)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": ""})
		req := httptest.NewRequest("POST", "/api/v1/groups", body)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/groups", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleGetGroup(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing group", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUpdateGroup(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("PUT", "/api/v1/groups/bad-id", jsonBody(t, map[string]string{"name": "X"}))
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing group", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("PUT", "/api/v1/groups/"+uuid.New().String(), jsonBody(t, map[string]string{"name": "X"}))
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeleteGroup(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/groups/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing group", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/groups/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleListGroupMembers(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups/bad-id/members", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns empty list for valid group", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/groups/"+uuid.New().String()+"/members", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleAddGroupMember(t *testing.T) {
	t.Run("returns 400 for invalid group UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/groups/bad-id/members", jsonBody(t, map[string]string{"device_id": uuid.New().String()}))
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects nil device_id", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"device_id": uuid.Nil.String()})
		req := httptest.NewRequest("POST", "/api/v1/groups/"+uuid.New().String()+"/members", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("adds member successfully", func(t *testing.T) {
		ts := newTestServer(t)
		gid := uuid.New()
		did := uuid.New()
		body := jsonBody(t, map[string]string{"device_id": did.String()})
		req := httptest.NewRequest("POST", "/api/v1/groups/"+gid.String()+"/members", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusNoContent, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "group.add_member", ts.auditLogger.events[0].Action)
	})
}

func TestHandleRemoveGroupMember(t *testing.T) {
	t.Run("returns 400 for invalid group UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/groups/bad-id/members/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid device UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/groups/"+uuid.New().String()+"/members/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("removes member successfully", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/groups/"+uuid.New().String()+"/members/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

// --- Compliance Handler Tests ---

func TestHandleComplianceSummary(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns summary", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/compliance", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleDeviceCompliance(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/bad-id/compliance", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns compliance for device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/"+uuid.New().String()+"/compliance", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleEvaluateDeviceCompliance(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/devices/bad-id/compliance/evaluate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/devices/"+uuid.New().String()+"/compliance/evaluate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("evaluates compliance for existing device", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel:    models.BaseModel{ID: id},
			EnterpriseID: testUser().EnterpriseID,
			Platform:     models.PlatformMacOS,
		})
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id.String()+"/compliance/evaluate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "compliance.evaluate", ts.auditLogger.events[0].Action)
	})
}

// --- Effective Policies Handler Test ---

func TestHandleGetDeviceEffectivePolicies(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/bad-id/effective-policies", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing device", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/devices/"+uuid.New().String()+"/effective-policies", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns effective policies", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
			BaseModel:    models.BaseModel{ID: id},
			EnterpriseID: testUser().EnterpriseID,
		})
		req := httptest.NewRequest("GET", "/api/v1/devices/"+id.String()+"/effective-policies", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- Report Handler Tests ---

func TestHandleDeviceReport_Full(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/devices", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns JSON report", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.reportService = &mockReportService{
			devices: []reporting.DeviceRow{{ID: uuid.New(), Platform: "macos", Name: "Test", Status: "enrolled"}},
		}
		req := httptest.NewRequest("GET", "/api/v1/reports/devices", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns CSV report", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.reportService = &mockReportService{
			devices: []reporting.DeviceRow{{ID: uuid.New(), Platform: "macos", Name: "Test", Status: "enrolled"}},
		}
		req := httptest.NewRequest("GET", "/api/v1/reports/devices?format=csv", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	})

	t.Run("handles service error", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.reportService = &mockReportService{err: fmt.Errorf("db error")}
		req := httptest.NewRequest("GET", "/api/v1/reports/devices", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleComplianceReport(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/compliance", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns JSON report", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/compliance", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns CSV report", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/compliance?format=csv", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	})
}

func TestHandleEnrollmentReport(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/enrollments", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns report with default days", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/enrollments", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("accepts days parameter", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/reports/enrollments?days=7", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- User Handler Tests ---

func TestHandleListUsers(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns users", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleGetUser(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/users/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing user", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/users/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleUpdateUser(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("PUT", "/api/v1/users/bad-id", jsonBody(t, map[string]string{"full_name": "X"}))
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing user", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("PUT", "/api/v1/users/"+uuid.New().String(), jsonBody(t, map[string]string{"full_name": "X"}))
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeactivateUser(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/users/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("deactivates user", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/users/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNoContent, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "user.deactivate", ts.auditLogger.events[0].Action)
	})
}

// --- Token Handler Tests ---

func TestHandleCreateToken(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "my-token"})
		req := httptest.NewRequest("POST", "/api/v1/tokens", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("creates token", func(t *testing.T) {
		ts := newTestServer(t)
		user := testUser()
		userID := uuid.MustParse(user.ID)

		// Seed the mock user repo with a user matching the caller's ID
		ts.userRepo.users = append(ts.userRepo.users, &models.User{
			BaseModel: models.BaseModel{ID: userID},
			Email:     user.Email,
			Role:      "admin",
		})

		body := jsonBody(t, map[string]string{"name": "my-token"})
		req := httptest.NewRequest("POST", "/api/v1/tokens", body)
		w := ts.doWithAuth(req, user)
		assert.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, ts.auditLogger.events, 1)
		assert.Equal(t, "token.create", ts.auditLogger.events[0].Action)
	})
}

func TestHandleRevokeToken(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/tokens/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Policy Versioning Handler Tests ---

func TestHandleListPolicyVersions(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/bad-id/versions", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns empty versions list", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/"+uuid.New().String()+"/versions", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleRollbackPolicy(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]int{"version": 1})
		req := httptest.NewRequest("POST", "/api/v1/policies/bad-id/rollback", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects version < 1", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]int{"version": 0})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+uuid.New().String()+"/rollback", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing version", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]int{"version": 99})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+uuid.New().String()+"/rollback", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleTranslatePolicy(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/bad-id/translate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing policy", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/"+uuid.New().String()+"/translate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("translates all platforms", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel:    models.BaseModel{ID: id},
			Platform:     "macos",
			PolicyType:   "wifi",
			PolicyConfig: models.JSONB{"ssid": "Test"},
		})
		req := httptest.NewRequest("GET", "/api/v1/policies/"+id.String()+"/translate", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("translates specific platform", func(t *testing.T) {
		ts := newTestServer(t)
		id := uuid.New()
		ts.policyRepo.policies = append(ts.policyRepo.policies, &models.Policy{
			BaseModel:    models.BaseModel{ID: id},
			Platform:     "macos",
			PolicyType:   "wifi",
			PolicyConfig: models.JSONB{"ssid": "Test"},
		})
		req := httptest.NewRequest("GET", "/api/v1/policies/"+id.String()+"/translate?platform=windows", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleListPolicyTemplates(t *testing.T) {
	t.Run("returns 401 without auth", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policy-templates", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns empty templates", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policy-templates", nil)
		w := ts.doWithAuth(req, testUser())
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleClonePolicyTemplate(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": "Clone"})
		req := httptest.NewRequest("POST", "/api/v1/policy-templates/bad-id/clone", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"name": ""})
		req := httptest.NewRequest("POST", "/api/v1/policy-templates/"+uuid.New().String()+"/clone", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Policy Assignment Handler Tests ---

func TestHandleAssignPolicyToTarget(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]interface{}{"target_type": "device", "target_id": uuid.New(), "priority": 1})
		req := httptest.NewRequest("POST", "/api/v1/policies/bad-id/assignments", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("creates assignment", func(t *testing.T) {
		ts := newTestServer(t)
		pid := uuid.New()
		body := jsonBody(t, map[string]interface{}{"target_type": "device", "target_id": uuid.New(), "priority": 1})
		req := httptest.NewRequest("POST", "/api/v1/policies/"+pid.String()+"/assignments", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestHandleUnassignPolicyFromTarget(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/policy-assignments/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing assignment", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("DELETE", "/api/v1/policy-assignments/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleListPolicyAssignments(t *testing.T) {
	t.Run("returns 400 for invalid UUID", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/bad-id/assignments", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns assignments", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/api/v1/policies/"+uuid.New().String()+"/assignments", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- Version Handler Test ---

func TestHandleVersion(t *testing.T) {
	t.Run("returns version info", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("GET", "/version", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "version")
		assert.Contains(t, w.Body.String(), "1.0.0")
	})
}

// --- Login Handler Tests ---

func TestHandleLogin(t *testing.T) {
	t.Run("rejects empty body", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects missing username", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"username": "", "password": "pass123"})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects missing password", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"username": "admin@test.com", "password": ""})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 401 for invalid credentials", func(t *testing.T) {
		// Point Keycloak config at a mock server that rejects login
		mockKC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		defer mockKC.Close()

		ts := newTestServer(t)
		ts.server.config.Keycloak.URL = mockKC.URL
		ts.server.config.Keycloak.Realm = "test"

		body := jsonBody(t, map[string]string{"username": "admin@test.com", "password": "wrong"})
		req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Refresh Handler Tests ---

func TestHandleRefresh(t *testing.T) {
	t.Run("rejects empty body", func(t *testing.T) {
		ts := newTestServer(t)
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects empty refresh_token", func(t *testing.T) {
		ts := newTestServer(t)
		body := jsonBody(t, map[string]string{"refresh_token": ""})
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects too-long refresh_token", func(t *testing.T) {
		ts := newTestServer(t)
		longToken := make([]byte, 2049)
		for i := range longToken {
			longToken[i] = 'a'
		}
		body := jsonBody(t, map[string]string{"refresh_token": string(longToken)})
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 401 for invalid refresh token", func(t *testing.T) {
		mockKC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		defer mockKC.Close()

		ts := newTestServer(t)
		ts.server.config.Keycloak.URL = mockKC.URL
		ts.server.config.Keycloak.Realm = "test"

		body := jsonBody(t, map[string]string{"refresh_token": "expired-token"})
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", body)
		w := ts.do(req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Platform Handler Tests (additional coverage) ---

func TestHandleWindowsPolicyService(t *testing.T) {
	t.Run("returns SOAP policy response", func(t *testing.T) {
		ts := newTestServer(t)
		// Register the route
		ts.server.router.HandleFunc("/EnrollmentServer/Policy.svc", ts.server.handleWindowsPolicyService).Methods("POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Policy.svc", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/soap+xml; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "GetPoliciesResponse")
	})
}

func TestHandleAndroidEnrollmentToken(t *testing.T) {
	t.Run("returns 400 for invalid enterprise ID", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/api/v1/android/enrollment-token/{enterprise_id}", ts.server.handleAndroidEnrollmentToken).Methods("POST")

		req := httptest.NewRequest("POST", "/api/v1/android/enrollment-token/bad-id", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for missing enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/api/v1/android/enrollment-token/{enterprise_id}", ts.server.handleAndroidEnrollmentToken).Methods("POST")

		req := httptest.NewRequest("POST", "/api/v1/android/enrollment-token/"+uuid.New().String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("generates token for valid enterprise", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/api/v1/android/enrollment-token/{enterprise_id}", ts.server.handleAndroidEnrollmentToken).Methods("POST")

		id := uuid.New()
		ts.enterpriseRepo.enterprises = append(ts.enterpriseRepo.enterprises, &models.Enterprise{
			BaseModel: models.BaseModel{ID: id},
			Name:      "Test",
			Slug:      "test",
		})

		req := httptest.NewRequest("POST", "/api/v1/android/enrollment-token/"+id.String(), nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "token_id")
	})
}

func TestHandleWindowsDiscoveryService(t *testing.T) {
	t.Run("returns 200 for empty body (GET probe)", func(t *testing.T) {
		ts := newTestServer(t)
		ts.server.router.HandleFunc("/EnrollmentServer/Discovery.svc", ts.server.handleWindowsDiscoveryService).Methods("GET", "POST")

		req := httptest.NewRequest("POST", "/EnrollmentServer/Discovery.svc", nil)
		w := ts.do(req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
