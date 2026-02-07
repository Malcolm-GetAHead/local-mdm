package validation_test

import (
	"testing"

	"github.com/malcolm-getahead/local-mdm/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello", "hello"},
		{"with tags", "<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"with quotes", `"test"`, "&#34;test&#34;"},
		{"with ampersand", "A&B", "A&amp;B"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.SanitizeHTML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"safe path", "files/document.pdf", "files/document.pdf"},
		{"parent traversal", "../etc/passwd", ""},
		{"absolute path", "/etc/passwd", ""},
		{"multiple traversal", "../../secret", ""},
		{"clean path", "files/../document.pdf", "document.pdf"},
		{"empty", "", "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.SanitizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string // strings that should NOT be in output
	}{
		{"single quote", "test'value", []string{"'"}},
		{"double quote", `test"value`, []string{`"`}},
		{"semicolon", "test;DROP TABLE", []string{";"}},
		{"comment", "test--comment", []string{"--"}},
		{"DROP keyword", "DROP TABLE users", []string{"DROP"}},
		{"INSERT keyword", "INSERT INTO users", []string{"INSERT"}},
		{"safe input", "testvalue123", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.SanitizeSQL(tt.input)
			for _, forbidden := range tt.contains {
				assert.NotContains(t, result, forbidden)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{"valid email", "user@example.com", true},
		{"valid with subdomain", "user@mail.example.com", true},
		{"missing @", "userexample.com", false},
		{"missing domain", "user@", false},
		{"missing dot", "user@example", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.ValidateEmail(tt.email)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name  string
		uuid  string
		valid bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID v4", "123e4567-e89b-12d3-a456-426614174000", true},
		{"too short", "550e8400-e29b-41d4-a716", false},
		{"too long", "550e8400-e29b-41d4-a716-446655440000-extra", false},
		{"wrong format", "550e8400e29b41d4a716446655440000", false},
		{"missing dashes", "550e8400-e29b-41d4a716-446655440000", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.ValidateUUID(tt.uuid)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"equal to max", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello"},
		{"empty string", "", 10, ""},
		{"zero length", "hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.TruncateString(tt.input, tt.maxLength)
			assert.Equal(t, tt.expected, result)
		})
	}
}
