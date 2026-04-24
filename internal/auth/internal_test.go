package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateJWKSURL_SSRF(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://keycloak.example.com/certs", false},
		{"valid localhost http", "http://localhost:8180/certs", false},
		{"valid 127.0.0.1", "http://127.0.0.1:8180/certs", false},
		{"blocks AWS metadata", "http://169.254.169.254/latest/meta-data/", true},
		{"blocks private 10.x", "http://10.0.0.1/certs", true},
		{"blocks private 192.168", "http://192.168.1.1/certs", true},
		{"blocks private 172.16", "http://172.16.0.1/certs", true},
		{"invalid scheme", "ftp://keycloak.example.com/certs", true},
		{"blocks link-local", "http://169.254.1.1/certs", true},
		{"blocks metadata.google.internal", "http://metadata.google.internal/computeMetadata/v1/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJWKSURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err, "expected error for %s", tt.url)
			} else {
				assert.NoError(t, err, "expected no error for %s", tt.url)
			}
		})
	}
}

func TestParseRSAPublicKey_NoX5c(t *testing.T) {
	key := JWK{
		Kty: "RSA",
		Kid: "test-key",
		Alg: "RS256",
		N:   "test",
		E:   "AQAB",
	}
	_, err := parseRSAPublicKey(key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "x5c not supported")
}
