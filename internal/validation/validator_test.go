package validation

import (
	"testing"
)

func TestValidator(t *testing.T) {
	t.Run("required_field", func(t *testing.T) {
		v := New()
		v.Required("username", "")
		
		if v.Valid() {
			t.Error("Expected validation to fail for empty required field")
		}
		
		if v.Errors()["username"] != "required" {
			t.Errorf("Expected 'required' error, got %s", v.Errors()["username"])
		}
	})

	t.Run("required_field_with_value", func(t *testing.T) {
		v := New()
		v.Required("username", "test")
		
		if !v.Valid() {
			t.Error("Expected validation to pass for non-empty required field")
		}
	})

	t.Run("min_length", func(t *testing.T) {
		v := New()
		v.MinLength("password", "abc", 8)
		
		if v.Valid() {
			t.Error("Expected validation to fail for short password")
		}
	})

	t.Run("max_length", func(t *testing.T) {
		v := New()
		v.MaxLength("username", "verylongusernamethatexceedsthelimit", 10)
		
		if v.Valid() {
			t.Error("Expected validation to fail for long username")
		}
	})

	t.Run("email_valid", func(t *testing.T) {
		v := New()
		v.Email("email", "test@example.com")
		
		if !v.Valid() {
			t.Error("Expected validation to pass for valid email")
		}
	})

	t.Run("email_invalid", func(t *testing.T) {
		v := New()
		v.Email("email", "invalid-email")
		
		if v.Valid() {
			t.Error("Expected validation to fail for invalid email")
		}
	})

	t.Run("uuid_valid", func(t *testing.T) {
		v := New()
		v.UUID("id", "550e8400-e29b-41d4-a716-446655440000")
		
		if !v.Valid() {
			t.Error("Expected validation to pass for valid UUID")
		}
	})

	t.Run("uuid_invalid", func(t *testing.T) {
		v := New()
		v.UUID("id", "not-a-uuid")
		
		if v.Valid() {
			t.Error("Expected validation to fail for invalid UUID")
		}
	})

	t.Run("one_of_valid", func(t *testing.T) {
		v := New()
		v.OneOf("platform", "macos", []string{"windows", "macos", "android"})
		
		if !v.Valid() {
			t.Error("Expected validation to pass for valid option")
		}
	})

	t.Run("one_of_invalid", func(t *testing.T) {
		v := New()
		v.OneOf("platform", "linux", []string{"windows", "macos", "android"})
		
		if v.Valid() {
			t.Error("Expected validation to fail for invalid option")
		}
	})

	t.Run("pattern_valid", func(t *testing.T) {
		v := New()
		v.Pattern("serial", "ABC123", "^[A-Z0-9]+$", "must be alphanumeric uppercase")
		
		if !v.Valid() {
			t.Error("Expected validation to pass for valid pattern")
		}
	})

	t.Run("pattern_invalid", func(t *testing.T) {
		v := New()
		v.Pattern("serial", "abc-123", "^[A-Z0-9]+$", "must be alphanumeric uppercase")
		
		if v.Valid() {
			t.Error("Expected validation to fail for invalid pattern")
		}
	})

	t.Run("multiple_errors", func(t *testing.T) {
		v := New()
		v.Required("username", "")
		v.Required("password", "")
		v.Email("email", "invalid")
		
		if v.Valid() {
			t.Error("Expected validation to fail")
		}
		
		if len(v.Errors()) != 3 {
			t.Errorf("Expected 3 errors, got %d", len(v.Errors()))
		}
	})

	t.Run("error_message", func(t *testing.T) {
		v := New()
		v.Required("username", "")
		
		err := v.Error()
		if err == nil {
			t.Error("Expected error message")
		}
	})

	t.Run("no_error_when_valid", func(t *testing.T) {
		v := New()
		v.Required("username", "test")
		
		err := v.Error()
		if err != nil {
			t.Errorf("Expected no error for valid input, got: %v", err)
		}
	})

	t.Run("one_of_empty_value", func(t *testing.T) {
		v := New()
		v.OneOf("platform", "", []string{"windows", "macos"})
		
		if !v.Valid() {
			t.Error("Expected empty value to be skipped in OneOf")
		}
	})

	t.Run("pattern_empty_value", func(t *testing.T) {
		v := New()
		v.Pattern("serial", "", "^[A-Z]+$", "must be uppercase")
		
		if !v.Valid() {
			t.Error("Expected empty value to be skipped in Pattern")
		}
	})
}
