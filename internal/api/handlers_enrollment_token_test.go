package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/windows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateEnrollmentToken(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	body := `{"enterprise_id":"` + ent.ID.String() + `","description":"test batch","max_uses":5,"expires_in":"24h"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	// Response is wrapped in "data" by respondJSON
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		data = resp // might be direct
	}
	assert.NotEmpty(t, data["token"])
	assert.NotEmpty(t, data["email"])
	assert.Contains(t, data["email"].(string), "@localmdm.local")
}

func TestHandleCreateEnrollmentToken_MissingEnterprise(t *testing.T) {
	ts := newTestServer(t)
	body := `{"enterprise_id":"` + uuid.New().String() + `","description":"test"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleCreateEnrollmentToken_InvalidMaxUses(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	body := `{"enterprise_id":"` + ent.ID.String() + `","max_uses":0}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleListEnrollmentTokens(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	maxUses := 10
	ts.enrollmentTokenRepo.Create(nil, &models.EnrollmentToken{
		EnterpriseID: ent.ID, Token: "tok1", Description: "first",
		MaxUses: &maxUses, UsesRemaining: &maxUses, ExpiresAt: time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/api/v1/enrollment-tokens?enterprise_id="+ent.ID.String(), nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandleListEnrollmentTokens_MissingEnterpriseID(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/enrollment-tokens", nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRevokeEnrollmentToken(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	tok := &models.EnrollmentToken{
		EnterpriseID: ent.ID, Token: "revokeme", ExpiresAt: time.Now().Add(time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("DELETE", "/api/v1/enrollment-tokens/"+tok.ID.String(), nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify revoked
	fetched, _ := ts.enrollmentTokenRepo.GetByToken(nil, "revokeme")
	assert.NotNil(t, fetched.RevokedAt)
	assert.Equal(t, models.EnrollmentTokenStatusRevoked, fetched.Status)
}

func TestHandleRevokeEnrollmentToken_NotFound(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/enrollment-tokens/"+uuid.New().String(), nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestValidateEnrollmentToken_Valid(t *testing.T) {
	ts := newTestServer(t)
	maxUses := 5
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "validtoken",
		MaxUses: &maxUses, UsesRemaining: &maxUses,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "validtoken")
	assert.NotNil(t, result)
	assert.Empty(t, errMsg)
	assert.Equal(t, tok.EnterpriseID, result.EnterpriseID)
}

func TestValidateEnrollmentToken_Expired(t *testing.T) {
	ts := newTestServer(t)
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "expiredtoken",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "expiredtoken")
	assert.Nil(t, result)
	assert.Contains(t, errMsg, "expired")

	// Verify status was updated to expired
	fetched, _ := ts.enrollmentTokenRepo.GetByToken(nil, "expiredtoken")
	assert.Equal(t, models.EnrollmentTokenStatusExpired, fetched.Status)
}

func TestValidateEnrollmentToken_AlreadyExpiredStatus(t *testing.T) {
	ts := newTestServer(t)
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "alreadyexpired",
		ExpiresAt: time.Now().Add(-time.Hour),
		Status:    models.EnrollmentTokenStatusExpired,
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "alreadyexpired")
	assert.Nil(t, result)
	assert.Contains(t, errMsg, "expired")
}

func TestValidateEnrollmentToken_Revoked(t *testing.T) {
	ts := newTestServer(t)
	now := time.Now()
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "revokedtoken",
		ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &now,
		Status: models.EnrollmentTokenStatusRevoked,
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "revokedtoken")
	assert.Nil(t, result)
	assert.Contains(t, errMsg, "revoked")
}

func TestValidateEnrollmentToken_Exhausted(t *testing.T) {
	ts := newTestServer(t)
	zero := 0
	maxUses := 5
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "exhaustedtoken",
		MaxUses: &maxUses, UsesRemaining: &zero,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "exhaustedtoken")
	assert.Nil(t, result)
	assert.Contains(t, errMsg, "no remaining uses")
}

func TestValidateEnrollmentToken_NotFound(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "nonexistent")
	assert.Nil(t, result)
	assert.Empty(t, errMsg) // empty means not found (not an error, just not a token)
}

func TestValidateEnrollmentToken_UnlimitedUses(t *testing.T) {
	ts := newTestServer(t)
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "unlimitedtoken",
		ExpiresAt: time.Now().Add(time.Hour),
		// MaxUses and UsesRemaining are nil = unlimited
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("GET", "/", nil)
	result, errMsg := ts.server.validateEnrollmentToken(req, "unlimitedtoken")
	assert.NotNil(t, result)
	assert.Empty(t, errMsg)
}

func TestRespondSOAPFault(t *testing.T) {
	rr := httptest.NewRecorder()
	respondSOAPFault(rr, "s:Sender", "test error message")
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/soap+xml")
	assert.Contains(t, rr.Body.String(), "test error message")
	assert.Contains(t, rr.Body.String(), "s:Sender")
}

func TestDecrementUses(t *testing.T) {
	ts := newTestServer(t)
	uses := 3
	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.New(), Token: "dectest",
		MaxUses: &uses, UsesRemaining: &uses,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	err := ts.enrollmentTokenRepo.DecrementUses(nil, tok.ID)
	require.NoError(t, err)

	fetched, _ := ts.enrollmentTokenRepo.GetByToken(nil, "dectest")
	assert.Equal(t, 2, *fetched.UsesRemaining)
}

// ── Additional coverage: handleCreateEnrollmentToken ──

func TestHandleCreateEnrollmentToken_InvalidJSON(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCreateEnrollmentToken_InvalidEnterpriseID(t *testing.T) {
	ts := newTestServer(t)
	body := `{"enterprise_id":"not-a-uuid"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCreateEnrollmentToken_InvalidExpiresIn(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	body := `{"enterprise_id":"` + ent.ID.String() + `","expires_in":"not-a-duration"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCreateEnrollmentToken_ExpiresInTooShort(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test"}
	ts.enterpriseRepo.Create(nil, ent)

	body := `{"enterprise_id":"` + ent.ID.String() + `","expires_in":"5s"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCreateEnrollmentToken_WithAuthContext(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test-auth"}
	ts.enterpriseRepo.Create(nil, ent)

	// Create a user in the mock repo so created_by FK check passes
	user := &models.User{Email: "admin@test.com", Role: "admin", EnterpriseID: ent.ID, IsActive: true}
	ts.userRepo.Create(nil, user)

	body := `{"enterprise_id":"` + ent.ID.String() + `","description":"with auth"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.doWithAuth(req, &auth.AuthUser{
		ID: user.ID.String(), Email: "admin@test.com", Roles: []string{"admin"}, EnterpriseID: ent.ID,
	})

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["macos_enroll_url"])
	assert.Equal(t, "active", data["status"])
}

func TestHandleCreateEnrollmentToken_DefaultExpiry(t *testing.T) {
	ts := newTestServer(t)
	ent := &models.Enterprise{Name: "Test", Slug: "test-default"}
	ts.enterpriseRepo.Create(nil, ent)

	body := `{"enterprise_id":"` + ent.ID.String() + `"}`
	req := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := ts.do(req)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestHandleListEnrollmentTokens_InvalidEnterpriseID(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/enrollment-tokens?enterprise_id=bad", nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRevokeEnrollmentToken_InvalidID(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/enrollment-tokens/not-a-uuid", nil)
	rr := ts.do(req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ── Additional coverage: Windows enrollment with tokens ──

func TestHandleWindowsEnrollment_ExpiredToken(t *testing.T) {
	ts := newTestServer(t)
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	require.NoError(t, err)
	ts.server.certService = certs.NewCertificateService(ca, &mockCertStore{})
	ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

	ent := &models.Enterprise{Name: "Test", Slug: "test-exp"}
	ts.enterpriseRepo.Create(nil, ent)
	ts.enrollmentTokenRepo.Create(nil, &models.EnrollmentToken{
		EnterpriseID: ent.ID, Token: "expiredwin", ExpiresAt: time.Now().Add(-time.Hour),
	})

	body := buildEnrollmentSOAPWithEmail(t, "expiredwin@localmdm.local")
	req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	w := ts.do(req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}

func TestHandleWindowsEnrollment_RevokedToken(t *testing.T) {
	ts := newTestServer(t)
	dir := t.TempDir()
	ca, err := certs.GenerateCA(dir+"/ca.crt", dir+"/ca.key")
	require.NoError(t, err)
	ts.server.certService = certs.NewCertificateService(ca, &mockCertStore{})
	ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

	ent := &models.Enterprise{Name: "Test", Slug: "test-rev"}
	ts.enterpriseRepo.Create(nil, ent)
	now := time.Now()
	ts.enrollmentTokenRepo.Create(nil, &models.EnrollmentToken{
		EnterpriseID: ent.ID, Token: "revokedwin", ExpiresAt: time.Now().Add(time.Hour),
		RevokedAt: &now, Status: models.EnrollmentTokenStatusRevoked,
	})

	body := buildEnrollmentSOAPWithEmail(t, "revokedwin@localmdm.local")
	req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	w := ts.do(req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "revoked")
}

func TestHandleWindowsEnrollment_NoUsernameToken(t *testing.T) {
	ts := newTestServer(t)
	ts.server.router.HandleFunc("/EnrollmentServer/Enrollment.svc", ts.server.handleWindowsEnrollmentService).Methods("POST")

	// SOAP without Security/UsernameToken header
	csr := windows.GenerateTestCSR(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://www.w3.org/2005/08/addressing"
            xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RST/wstep</a:Action>
    <a:MessageID>urn:uuid:no-auth</a:MessageID>
  </s:Header>
  <s:Body>
    <wst:RequestSecurityToken>
      <wst:TokenType>http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken</wst:TokenType>
      <wst:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</wst:RequestType>
      <wsse:BinarySecurityToken xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
                                ValueType="http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS10"
                                EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary">` + csr + `</wsse:BinarySecurityToken>
    </wst:RequestSecurityToken>
  </s:Body>
</s:Envelope>`
	req := httptest.NewRequest("POST", "/EnrollmentServer/Enrollment.svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	w := ts.do(req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment token required")
}

// ── Additional coverage: web handlers ──

func TestWebEnrollmentTokens_ListPage(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	dash := ts.server.router.PathPrefix("/dashboard").Subrouter()
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokens).Methods("GET")

	ent := ts.server.config
	_ = ent
	maxUses := 5
	ts.enrollmentTokenRepo.Create(nil, &models.EnrollmentToken{
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Token: "webtest1", Description: "Web test token",
		MaxUses: &maxUses, UsesRemaining: &maxUses,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest("GET", "/dashboard/enrollment-tokens", nil)
	rr := ts.doWithSession(req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Enrollment Tokens")
	assert.Contains(t, body, "webtest1@localmdm.local")
	assert.Contains(t, body, "Web test token")
	assert.Contains(t, body, "Create Token")
}

func TestWebEnrollmentTokens_CreateRedirect(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	dash := ts.server.router.PathPrefix("/dashboard").Subrouter()
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokens).Methods("GET")
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokenCreate).Methods("POST")

	req := httptest.NewRequest("POST", "/dashboard/enrollment-tokens", strings.NewReader("description=Go+test&max_uses=3&expires_in=1h"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := ts.doWithSession(req)

	// Non-HTMX POST should redirect
	assert.Equal(t, http.StatusFound, rr.Code)
	loc := rr.Header().Get("Location")
	assert.Contains(t, loc, "/dashboard/enrollment-tokens?created=")

	// Verify token was created
	tokens, total, _ := ts.enrollmentTokenRepo.List(nil, uuid.MustParse("00000000-0000-0000-0000-000000000001"), 10, 0)
	assert.Equal(t, 1, total)
	assert.Equal(t, "Go test", tokens[0].Description)
	assert.Equal(t, 3, *tokens[0].MaxUses)
}

func TestWebEnrollmentTokens_CreateHTMX(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	dash := ts.server.router.PathPrefix("/dashboard").Subrouter()
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokens).Methods("GET")
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokenCreate).Methods("POST")

	req := httptest.NewRequest("POST", "/dashboard/enrollment-tokens", strings.NewReader("description=HTMX+test&expires_in=24h"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "page-content")
	rr := ts.doWithSession(req)

	// HTMX POST should render page directly with HX-Push-Url
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("HX-Push-Url"), "?created=")
	assert.Contains(t, rr.Body.String(), "Token created")
	assert.Contains(t, rr.Body.String(), "@localmdm.local")
}

func TestWebEnrollmentTokens_Revoke(t *testing.T) {
	ts := newTestServerWithTemplates(t)
	dash := ts.server.router.PathPrefix("/dashboard").Subrouter()
	dash.HandleFunc("/enrollment-tokens", ts.server.handleWebEnrollmentTokens).Methods("GET")
	dash.HandleFunc("/enrollment-tokens/{id}/revoke", ts.server.handleWebEnrollmentTokenRevoke).Methods("POST")

	tok := &models.EnrollmentToken{
		EnterpriseID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Token: "webrevoke", ExpiresAt: time.Now().Add(time.Hour),
	}
	ts.enrollmentTokenRepo.Create(nil, tok)

	req := httptest.NewRequest("POST", "/dashboard/enrollment-tokens/"+tok.ID.String()+"/revoke", nil)
	rr := ts.doWithSession(req)

	assert.Equal(t, http.StatusFound, rr.Code)

	fetched, _ := ts.enrollmentTokenRepo.GetByToken(nil, "webrevoke")
	assert.Equal(t, models.EnrollmentTokenStatusRevoked, fetched.Status)
}
