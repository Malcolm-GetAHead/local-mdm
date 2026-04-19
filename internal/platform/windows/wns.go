package windows

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WNSClient sends push notifications via Windows Notification Service.
type WNSClient struct {
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
	httpClient   *http.Client
}

// NewWNSClient creates a WNS push notification client.
func NewWNSClient(clientID, clientSecret string) *WNSClient {
	return &WNSClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Push sends a raw notification to trigger an MDM check-in.
// channelURI is the device's WNS channel URI stored during enrollment.
func (c *WNSClient) Push(ctx context.Context, channelURI string) error {
	if channelURI == "" {
		return fmt.Errorf("channel URI is empty")
	}

	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get WNS access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, channelURI, strings.NewReader(""))
	if err != nil {
		return fmt.Errorf("failed to create WNS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-WNS-Type", "wns/raw")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("WNS push failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WNS returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getAccessToken retrieves or refreshes the OAuth2 access token for WNS.
func (c *WNSClient) getAccessToken(ctx context.Context) (string, error) {
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	if c.clientID == "" || c.clientSecret == "" {
		return "", fmt.Errorf("WNS client credentials not configured")
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"scope":         {"notify.windows.com/.default"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://login.microsoftonline.com/common/oauth2/v2.0/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("WNS token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("WNS token request returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse token response — simple extraction without full JSON parsing
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Extract access_token from JSON response
	token := extractJSONString(string(body), "access_token")
	if token == "" {
		return "", fmt.Errorf("no access_token in WNS response")
	}

	c.accessToken = token
	c.tokenExpiry = time.Now().Add(50 * time.Minute) // tokens typically valid for 60 min
	return token, nil
}

// extractJSONString extracts a string value from a JSON object by key.
// Simple implementation — avoids importing encoding/json for a single field.
func extractJSONString(body, key string) string {
	search := fmt.Sprintf(`"%s":"`, key)
	idx := strings.Index(body, search)
	if idx < 0 {
		search = fmt.Sprintf(`"%s": "`, key)
		idx = strings.Index(body, search)
		if idx < 0 {
			return ""
		}
	}
	start := idx + len(search)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}
