package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/models"
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
