package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type OIDCValidator struct {
	issuerURL     string
	clientID      string
	jwksURL       string
	jwks          *JWKS
	jwksMutex     sync.RWMutex
	lastRefresh   time.Time
	refreshEvery  time.Duration
	refreshMutex  sync.Mutex
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Alg string   `json:"alg"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c,omitempty"`
}

type TokenClaims struct {
	jwt.RegisteredClaims
	Email        string                 `json:"email"`
	RealmAccess  map[string]interface{} `json:"realm_access"`
	Azp          string                 `json:"azp"`
	EnterpriseID string                 `json:"enterprise_id,omitempty"`
}

func NewOIDCValidator(issuerURL, clientID string) (*OIDCValidator, error) {
	v := &OIDCValidator{
		issuerURL:    issuerURL,
		clientID:     clientID,
		jwksURL:      fmt.Sprintf("%s/protocol/openid-connect/certs", issuerURL),
		refreshEvery: 1 * time.Hour,
	}
	
	if err := v.refreshJWKS(); err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	
	return v, nil
}

func (v *OIDCValidator) refreshJWKS() error {
	v.refreshMutex.Lock()
	defer v.refreshMutex.Unlock()
	
	// Double-check pattern: verify refresh is still needed after acquiring lock
	v.jwksMutex.RLock()
	needsRefresh := time.Since(v.lastRefresh) > v.refreshEvery
	v.jwksMutex.RUnlock()
	
	if !needsRefresh {
		return nil
	}
	
	// Create HTTP client with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}
	
	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}
	
	// Validate JWKS has keys
	if len(jwks.Keys) == 0 {
		return fmt.Errorf("JWKS contains no keys")
	}
	
	v.jwksMutex.Lock()
	v.jwks = &jwks
	v.lastRefresh = time.Now()
	v.jwksMutex.Unlock()
	
	return nil
}

func (v *OIDCValidator) ValidateToken(tokenString string) (*AuthUser, error) {
	// Check if JWKS refresh is needed (lock-free read)
	v.jwksMutex.RLock()
	needsRefresh := time.Since(v.lastRefresh) > v.refreshEvery
	v.jwksMutex.RUnlock()
	
	// Refresh JWKS if needed (non-blocking)
	if needsRefresh {
		go v.refreshJWKS()
	}
	
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		// Get key ID from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}
		
		// Find matching key in JWKS
		v.jwksMutex.RLock()
		defer v.jwksMutex.RUnlock()
		
		for _, key := range v.jwks.Keys {
			if key.Kid == kid {
				// Parse public key from JWK
				return parseRSAPublicKey(key)
			}
		}
		
		return nil, fmt.Errorf("key not found in JWKS")
	})
	
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	
	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	
	// Verify issuer
	if claims.Issuer != v.issuerURL {
		return nil, fmt.Errorf("invalid issuer")
	}
	
	// Extract roles
	roles := []string{}
	if realmAccess, ok := claims.RealmAccess["roles"].([]interface{}); ok {
		for _, role := range realmAccess {
			if roleStr, ok := role.(string); ok {
				roles = append(roles, roleStr)
			}
		}
	}
	
	// Parse enterprise ID
	var enterpriseID uuid.UUID
	if claims.EnterpriseID != "" {
		enterpriseID, _ = uuid.Parse(claims.EnterpriseID)
	}
	
	return &AuthUser{
		ID:           claims.Subject,
		Email:        claims.Email,
		Roles:        roles,
		EnterpriseID: enterpriseID,
	}, nil
}

func parseRSAPublicKey(key JWK) (interface{}, error) {
	// For simplicity, use x5c certificate if available
	if len(key.X5c) > 0 {
		return jwt.ParseRSAPublicKeyFromPEM([]byte(fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s\n-----END CERTIFICATE-----", key.X5c[0])))
	}
	
	// Otherwise would need to construct key from N and E
	return nil, fmt.Errorf("JWK without x5c not supported yet")
}

func ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}
	
	return parts[1], nil
}
