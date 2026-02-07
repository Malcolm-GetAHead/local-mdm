package validation

import (
	"html"
	"path/filepath"
	"strings"
)

// SanitizeHTML removes HTML tags and escapes special characters
func SanitizeHTML(input string) string {
	return html.EscapeString(input)
}

// SanitizePath prevents path traversal attacks
func SanitizePath(input string) string {
	// Remove any path traversal attempts
	cleaned := filepath.Clean(input)
	
	// Ensure no absolute paths or parent directory references
	if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
		return ""
	}
	
	return cleaned
}

// SanitizeSQL prevents SQL injection (note: use parameterized queries instead)
func SanitizeSQL(input string) string {
	// Remove common SQL injection patterns
	dangerous := []string{
		"'", "\"", ";", "--", "/*", "*/", "xp_", "sp_",
		"DROP", "INSERT", "UPDATE", "DELETE", "EXEC", "EXECUTE",
	}
	
	result := input
	for _, pattern := range dangerous {
		result = strings.ReplaceAll(result, pattern, "")
	}
	
	return result
}

// ValidateEmail performs basic email validation
func ValidateEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// ValidateUUID validates UUID format
func ValidateUUID(uuid string) bool {
	if len(uuid) != 36 {
		return false
	}
	
	// Basic UUID format check (8-4-4-4-12)
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		return false
	}
	
	return len(parts[0]) == 8 && len(parts[1]) == 4 && len(parts[2]) == 4 && 
		len(parts[3]) == 4 && len(parts[4]) == 12
}

// TruncateString limits string length
func TruncateString(input string, maxLength int) string {
	if len(input) <= maxLength {
		return input
	}
	return input[:maxLength]
}
