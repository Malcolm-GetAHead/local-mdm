package auth

import (
	"testing"
)

func TestLoginRequestValidation(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		req := LoginRequest{
			Username: "testuser",
			Password: "testpass",
		}
		
		if err := req.Validate(); err != nil {
			t.Errorf("Expected valid request, got error: %v", err)
		}
	})

	t.Run("missing_username", func(t *testing.T) {
		req := LoginRequest{
			Username: "",
			Password: "testpass",
		}
		
		if err := req.Validate(); err == nil {
			t.Error("Expected error for missing username")
		}
	})

	t.Run("missing_password", func(t *testing.T) {
		req := LoginRequest{
			Username: "testuser",
			Password: "",
		}
		
		if err := req.Validate(); err == nil {
			t.Error("Expected error for missing password")
		}
	})

	t.Run("username_too_long", func(t *testing.T) {
		req := LoginRequest{
			Username: string(make([]byte, 256)),
			Password: "testpass",
		}
		
		if err := req.Validate(); err == nil {
			t.Error("Expected error for username too long")
		}
	})

	t.Run("password_too_long", func(t *testing.T) {
		req := LoginRequest{
			Username: "testuser",
			Password: string(make([]byte, 129)),
		}
		
		if err := req.Validate(); err == nil {
			t.Error("Expected error for password too long")
		}
	})

	t.Run("edge_case_max_lengths", func(t *testing.T) {
		req := LoginRequest{
			Username: string(make([]byte, 255)), // Exactly 255
			Password: string(make([]byte, 128)), // Exactly 128
		}
		
		if err := req.Validate(); err != nil {
			t.Errorf("Expected valid request at max lengths, got error: %v", err)
		}
	})
}
