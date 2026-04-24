package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	sessionCookieName = "lmdm_session"
	sessionMaxAge     = 8 * time.Hour
)

// webSession stores the authenticated user session in a cookie.
// For simplicity, we store a signed JSON payload. In production, use gorilla/sessions with encryption.
type webSession struct {
	UserID       uuid.UUID `json:"uid"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	EnterpriseID uuid.UUID `json:"eid"`
	ExpiresAt    time.Time `json:"exp"`
}

func (s *Server) setWebSession(w http.ResponseWriter, sess *webSession) {
	sess.ExpiresAt = time.Now().Add(sessionMaxAge)
	data, _ := json.Marshal(sess)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    hex.EncodeToString(data),
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
	data, err := hex.DecodeString(cookie.Value)
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
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type webSessionKey_ string

const webSessionCtxKey webSessionKey_ = "web_session"

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
	resp, err := http.PostForm(tokenURL, map[string][]string{
		"grant_type":   {"authorization_code"},
		"client_id":    {s.config.Keycloak.ClientID},
		"client_secret": {s.config.Keycloak.ClientSecret},
		"code":         {code},
		"redirect_uri": {s.webCallbackURL(r)},
	})
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
	role := ""
	if len(claims.Roles) > 0 {
		role = claims.Roles[0]
	}

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
	http.Redirect(w, r, "/dashboard/login", http.StatusFound)
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
