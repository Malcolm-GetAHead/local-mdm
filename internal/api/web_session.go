package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/malcolm-getahead/local-mdm/internal/auth"
)

const (
	sessionCookieName = "lmdm_session"
	sessionMaxAge     = 8 * time.Hour
)

// webSession stores the authenticated user session in a cookie.
type webSession struct {
	UserID       uuid.UUID `json:"uid"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	EnterpriseID uuid.UUID `json:"eid"`
	ExpiresAt    time.Time `json:"exp"`
}

// sessionKey returns the HMAC signing key for dashboard sessions.
// Prefers dedicated session_secret; falls back to Keycloak client secret.
func (s *Server) sessionKey() []byte {
	if s.config.Keycloak.SessionSecret != "" {
		return []byte(s.config.Keycloak.SessionSecret)
	}
	return []byte(s.config.Keycloak.ClientSecret)
}

func (s *Server) signSession(data []byte) string {
	mac := hmac.New(sha256.New, s.sessionKey())
	mac.Write(data)
	sig := hex.EncodeToString(mac.Sum(nil))
	return hex.EncodeToString(data) + "." + sig
}

func (s *Server) verifySession(cookie string) ([]byte, error) {
	parts := splitOnce(cookie, '.')
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid session format")
	}
	data, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid session encoding")
	}
	mac := hmac.New(sha256.New, s.sessionKey())
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expected)) {
		return nil, fmt.Errorf("invalid session signature")
	}
	return data, nil
}

func splitOnce(s string, sep byte) []string {
	for i := range s {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func (s *Server) setWebSession(w http.ResponseWriter, sess *webSession) {
	sess.ExpiresAt = time.Now().Add(sessionMaxAge)
	data, _ := json.Marshal(sess)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(data),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

func (s *Server) getWebSession(r *http.Request) *webSession {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	data, err := s.verifySession(cookie.Value)
	if err != nil {
		return nil
	}
	var sess webSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return &sess
}

func (s *Server) clearWebSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// webAuthMiddleware checks for a valid session cookie and redirects to login if missing.
func (s *Server) webAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := s.getWebSession(r)
		if sess == nil {
			http.Redirect(w, r, "/dashboard/login", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), webSessionCtxKey, sess)
		authUser := &auth.AuthUser{
			ID:           sess.UserID.String(),
			Email:        sess.Email,
			Roles:        []string{sess.Role},
			EnterpriseID: sess.EnterpriseID,
		}
		ctx = auth.WithUser(ctx, authUser)

		// CSRF: generate token for GET, validate for POST
		if r.Method == "POST" {
			token := r.FormValue("_csrf")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}
			expected := generateCSRF(sess.UserID.String(), s.sessionKey())
			if !hmac.Equal([]byte(token), []byte(expected)) && r.Header.Get("HX-Request") == "" {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		csrfToken := generateCSRF(sess.UserID.String(), s.sessionKey())
		ctx = context.WithValue(ctx, csrfTokenKey, csrfToken)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateCSRF(userID string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("csrf:" + userID))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

type webSessionKey_ string

const webSessionCtxKey webSessionKey_ = "web_session"
const csrfTokenKey webSessionKey_ = "csrf_token"

func getSession(r *http.Request) *webSession {
	sess, _ := r.Context().Value(webSessionCtxKey).(*webSession)
	return sess
}

func sessionToUser(sess *webSession) *sessionUser {
	if sess == nil {
		return nil
	}
	return &sessionUser{
		ID:           sess.UserID,
		Email:        sess.Email,
		Role:         sess.Role,
		EnterpriseID: sess.EnterpriseID,
	}
}

// handleWebLogin redirects to Keycloak OIDC authorization endpoint.
func (s *Server) handleWebLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   300,
	})

	authURL := fmt.Sprintf("%s/protocol/openid-connect/auth?client_id=%s&response_type=code&redirect_uri=%s&scope=openid+email+profile&state=%s",
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		s.webCallbackURL(r),
		state,
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleWebCallback handles the OIDC callback from Keycloak.
func (s *Server) handleWebCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens via Keycloak token endpoint
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", s.config.Keycloak.IssuerURL())
	formData := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {s.config.Keycloak.ClientID},
		"client_secret": {s.config.Keycloak.ClientSecret},
		"code":          {code},
		"redirect_uri":  {s.webCallbackURL(r)},
	}
	tokenReq, err := http.NewRequestWithContext(r.Context(), "POST", tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		s.logger.Error("Token exchange request creation failed", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := tokenClient.Do(tokenReq)
	if err != nil {
		s.logger.Error("Token exchange failed", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil || tokenResp.AccessToken == "" {
		s.logger.Error("Token decode failed", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Use the existing auth middleware to validate the token and extract claims
	claims, err := s.authMiddleware.ValidateTokenDirect(tokenResp.AccessToken)
	if err != nil {
		s.logger.Error("Token validation failed", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	userID, _ := uuid.Parse(claims.ID)
	role := pickBestRole(claims.Roles)

	s.setWebSession(w, &webSession{
		UserID:       userID,
		Email:        claims.Email,
		Role:         role,
		EnterpriseID: claims.EnterpriseID,
	})

	http.Redirect(w, r, "/dashboard/", http.StatusFound)
}

func (s *Server) handleWebLogout(w http.ResponseWriter, r *http.Request) {
	s.clearWebSession(w)
	// Redirect to Keycloak's logout endpoint to end the SSO session
	logoutURL := fmt.Sprintf("%s/protocol/openid-connect/logout?client_id=%s&post_logout_redirect_uri=%s",
		s.config.Keycloak.IssuerURL(),
		s.config.Keycloak.ClientID,
		fmt.Sprintf("http://%s/dashboard/login", r.Host),
	)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

func (s *Server) webCallbackURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/dashboard/callback", scheme, r.Host)
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// pickBestRole returns the most privileged application role from a list.
func pickBestRole(roles []string) string {
	priority := map[string]int{"super_admin": 4, "admin": 3, "operator": 2, "viewer": 1}
	best, bestP := "viewer", 0
	for _, r := range roles {
		if p, ok := priority[r]; ok && p > bestP {
			best, bestP = r, p
		}
	}
	if bestP == 0 {
		return "viewer"
	}
	return best
}
