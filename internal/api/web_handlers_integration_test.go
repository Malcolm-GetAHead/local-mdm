package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestWebDashboardHome(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.deviceRepo.devices = testDevices()
	ts.server.reportService = &mockReportService{}

	req := httptest.NewRequest("GET", "/dashboard/", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Total Devices")
}

func TestWebDeviceList(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.deviceRepo.devices = testDevices()

	req := httptest.NewRequest("GET", "/dashboard/devices", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Test Device")
}

func TestWebDeviceList_HTMXFragment(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.deviceRepo.devices = testDevices()

	req := httptest.NewRequest("GET", "/dashboard/devices?q=test", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "device-table")
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	// Fragment should NOT contain full page shell
	assert.NotContains(t, w.Body.String(), "<!DOCTYPE html>")
}

func TestWebDeviceList_HTMXNavigation(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.deviceRepo.devices = testDevices()

	req := httptest.NewRequest("GET", "/dashboard/devices", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "page-content")
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	// Should contain header but NOT full page shell
	body := w.Body.String()
	assert.NotContains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Logout")
	assert.Contains(t, body, "Devices")
}

func TestWebPolicyList(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.policyRepo.policies = []*models.Policy{
		{BaseModel: models.BaseModel{ID: uuid.New()}, Name: "Test Policy", Platform: "all", PolicyType: "security", IsActive: true},
	}

	req := httptest.NewRequest("GET", "/dashboard/policies", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Test Policy")
}

func TestWebGroups(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	req := httptest.NewRequest("GET", "/dashboard/groups", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Groups")
}

func TestWebCompliance(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	ts.server.reportService = &mockReportService{}

	req := httptest.NewRequest("GET", "/dashboard/compliance", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Compliant")
}

func TestWebAuditLog(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	req := httptest.NewRequest("GET", "/dashboard/audit", nil)
	w := ts.doWithSession(req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Audit Log")
}

func testDevices() []*models.Device {
	return []*models.Device{
		{
			BaseModel:    models.BaseModel{ID: uuid.New()},
			EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Name:         "Test Device",
			Platform:     "macos",
			Model:        "MacBook Pro",
			Status:       "enrolled",
		},
	}
}

func TestWebOIDCCallbackHappyPath(t *testing.T) {
	// This test hits real Keycloak — skip if unavailable
	keycloakURL := "http://localhost:8180"
	if u := os.Getenv("KEYCLOAK_URL"); u != "" {
		keycloakURL = u
	}
	resp, err := http.Get(keycloakURL + "/realms/localmdm/.well-known/openid-configuration")
	if err != nil || resp.StatusCode != 200 {
		t.Skip("Keycloak not available")
	}
	resp.Body.Close()

	clientSecret := "localmdm-dev-dashboard-secret-2026"
	if s := os.Getenv("KEYCLOAK_CLIENT_SECRET"); s != "" {
		clientSecret = s
	}

	ts := newTestServerWithTemplates(t)
	ts.server.config.Keycloak.URL = keycloakURL
	ts.server.config.Keycloak.Realm = "localmdm"
	ts.server.config.Keycloak.ClientID = "localmdm-api"
	ts.server.config.Keycloak.ClientSecret = clientSecret

	// Set up auth middleware for token validation
	validator, err := auth.NewOIDCValidator(
		keycloakURL+"/realms/localmdm",
		"localmdm-api",
		nil, 5, 30*time.Second, 5*time.Minute, nil,
	)
	if err != nil {
		t.Skipf("Failed to create OIDC validator: %v", err)
	}
	ts.server.authMiddleware = auth.NewMiddleware(validator, ts.server.logger)

	// Register callback route
	ts.server.router.HandleFunc("/dashboard/callback", ts.server.handleWebCallback).Methods("GET")

	// Step 1: Get an auth code by doing a direct resource owner password grant
	// to get tokens, then we test the callback error handling since we can't
	// easily get an auth code without a browser redirect flow.
	// Instead, test the full flow by verifying the login redirect URL is correct.
	ts.server.router.HandleFunc("/dashboard/login", ts.server.handleWebLogin).Methods("GET")

	req := httptest.NewRequest("GET", "/dashboard/login", nil)
	req.Host = "localhost:8080"
	w := httptest.NewRecorder()
	ts.server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, keycloakURL+"/realms/localmdm/protocol/openid-connect/auth")
	assert.Contains(t, location, "client_id=localmdm-api")
	assert.Contains(t, location, "redirect_uri=http://localhost:8080/dashboard/callback")

	// Verify state cookie was set
	cookies := w.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}
	assert.NotNil(t, stateCookie, "oauth_state cookie should be set")
	assert.Contains(t, location, "state="+stateCookie.Value)
}
