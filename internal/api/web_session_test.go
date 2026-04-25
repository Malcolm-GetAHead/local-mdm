package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionSignAndVerify(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	data := []byte(`{"uid":"test"}`)
	signed := ts.server.signSession(data)

	// Valid signature
	got, err := ts.server.verifySession(signed)
	require.NoError(t, err)
	assert.Equal(t, data, got)

	// Tampered signature
	_, err = ts.server.verifySession(signed + "x")
	assert.Error(t, err)

	// Tampered data
	parts := strings.SplitN(signed, ".", 2)
	_, err = ts.server.verifySession("deadbeef." + parts[1])
	assert.Error(t, err)

	// No dot separator
	_, err = ts.server.verifySession("nodot")
	assert.Error(t, err)
}

func TestSessionKey_PrefersSessionSecret(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "client-secret"
	ts.server.config.Keycloak.SessionSecret = ""
	assert.Equal(t, []byte("client-secret"), ts.server.sessionKey())

	ts.server.config.Keycloak.SessionSecret = "dedicated-session-secret"
	assert.Equal(t, []byte("dedicated-session-secret"), ts.server.sessionKey())
}

func TestSetAndGetWebSession(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	sess := &webSession{
		UserID:       uuid.New(),
		Email:        "admin@test.com",
		Role:         "admin",
		EnterpriseID: uuid.New(),
	}

	// Set session
	w := httptest.NewRecorder()
	ts.server.setWebSession(w, sess)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, sessionCookieName, cookies[0].Name)
	assert.True(t, cookies[0].HttpOnly)

	// Get session back
	r, _ := http.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	got := ts.server.getWebSession(r)
	require.NotNil(t, got)
	assert.Equal(t, sess.Email, got.Email)
	assert.Equal(t, sess.Role, got.Role)
	assert.Equal(t, sess.UserID, got.UserID)
}

func TestGetWebSession_Expired(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	sess := &webSession{
		UserID:    uuid.New(),
		Email:     "admin@test.com",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	data, _ := json.Marshal(sess)
	signed := ts.server.signSession(data)

	r, _ := http.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signed})
	got := ts.server.getWebSession(r)
	assert.Nil(t, got, "expired session should return nil")
}

func TestGetWebSession_InvalidCookie(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	r, _ := http.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "garbage.data"})
	got := ts.server.getWebSession(r)
	assert.Nil(t, got, "invalid cookie should return nil")
}

func TestCSRFValidation_RejectsForgedPost(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	// Create a valid session
	sess := &webSession{
		UserID:       uuid.New(),
		Email:        "admin@test.com",
		Role:         "admin",
		EnterpriseID: uuid.New(),
	}
	w := httptest.NewRecorder()
	ts.server.setWebSession(w, sess)
	sessionCookie := w.Result().Cookies()[0]

	// POST without CSRF token should be rejected (non-HTMX)
	handler := ts.server.webAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r, _ := http.NewRequest("POST", "/dashboard/test", strings.NewReader("name=test"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRFValidation_AcceptsValidToken(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	sess := &webSession{
		UserID:       uuid.New(),
		Email:        "admin@test.com",
		Role:         "admin",
		EnterpriseID: uuid.New(),
	}
	w := httptest.NewRecorder()
	ts.server.setWebSession(w, sess)
	sessionCookie := w.Result().Cookies()[0]

	csrfToken := generateCSRF(sess.UserID.String(), ts.server.sessionKey())

	handler := ts.server.webAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r, _ := http.NewRequest("POST", "/dashboard/test", strings.NewReader("_csrf="+csrfToken+"&name=test"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRFValidation_HTMXRequestsExempt(t *testing.T) {
	ts := newTestServer(t)
	ts.server.config.Keycloak.ClientSecret = "test-secret-32chars-minimum!!"

	sess := &webSession{
		UserID:       uuid.New(),
		Email:        "admin@test.com",
		Role:         "admin",
		EnterpriseID: uuid.New(),
	}
	w := httptest.NewRecorder()
	ts.server.setWebSession(w, sess)
	sessionCookie := w.Result().Cookies()[0]

	handler := ts.server.webAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// HTMX POST without CSRF token should pass
	r, _ := http.NewRequest("POST", "/dashboard/test", strings.NewReader("name=test"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	r.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWebAuthMiddleware_RedirectsWithoutSession(t *testing.T) {
	ts := newTestServer(t)

	handler := ts.server.webAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r, _ := http.NewRequest("GET", "/dashboard/devices", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/dashboard/login")
}

func TestWebCallback_MissingState(t *testing.T) {
	ts := newTestServer(t)
	r, _ := http.NewRequest("GET", "/dashboard/callback?code=test", nil)
	w := httptest.NewRecorder()
	ts.server.handleWebCallback(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebCallback_MissingCode(t *testing.T) {
	ts := newTestServer(t)
	r, _ := http.NewRequest("GET", "/dashboard/callback?state=abc", nil)
	r.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	w := httptest.NewRecorder()
	ts.server.handleWebCallback(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebCallback_StateMismatch(t *testing.T) {
	ts := newTestServer(t)
	r, _ := http.NewRequest("GET", "/dashboard/callback?state=wrong&code=test", nil)
	r.AddCookie(&http.Cookie{Name: "oauth_state", Value: "expected"})
	w := httptest.NewRecorder()
	ts.server.handleWebCallback(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
