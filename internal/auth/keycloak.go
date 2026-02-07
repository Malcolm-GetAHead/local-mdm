package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Validate validates the login request
func (r *LoginRequest) Validate() error {
	if r.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(r.Username) > 255 {
		return fmt.Errorf("username must be at most 255 characters")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(r.Password) < 1 {
		return fmt.Errorf("password is required")
	}
	if len(r.Password) > 128 {
		return fmt.Errorf("password must be at most 128 characters")
	}
	return nil
}

type KeycloakClient struct {
	issuerURL string
	clientID  string
	clientSecret string
}

func NewKeycloakClient(issuerURL, clientID, clientSecret string) *KeycloakClient {
	return &KeycloakClient{
		issuerURL: issuerURL,
		clientID:  clientID,
		clientSecret: clientSecret,
	}
}

func (kc *KeycloakClient) Login(username, password string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", kc.issuerURL)
	
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", kc.clientID)
	data.Set("client_secret", kc.clientSecret)
	data.Set("username", username)
	data.Set("password", password)
	
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed: %s", string(body))
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	
	return &tokenResp, nil
}

func (kc *KeycloakClient) RefreshToken(refreshToken string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", kc.issuerURL)
	
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", kc.clientID)
	data.Set("client_secret", kc.clientSecret)
	data.Set("refresh_token", refreshToken)
	
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed: %s", string(body))
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	
	return &tokenResp, nil
}
