package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrRequired      = errors.New("field is required")
	ErrInvalidFormat = errors.New("invalid format")
	ErrTooShort      = errors.New("value too short")
	ErrTooLong       = errors.New("value too long")
)

// Validator provides input validation
type Validator struct {
	errors map[string]string
}

// New creates a new validator
func New() *Validator {
	return &Validator{
		errors: make(map[string]string),
	}
}

// Required validates that a field is not empty
func (v *Validator) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		v.errors[field] = "required"
	}
}

// MinLength validates minimum length
func (v *Validator) MinLength(field, value string, min int) {
	if len(value) < min {
		v.errors[field] = fmt.Sprintf("must be at least %d characters", min)
	}
}

// MaxLength validates maximum length
func (v *Validator) MaxLength(field, value string, max int) {
	if len(value) > max {
		v.errors[field] = fmt.Sprintf("must be at most %d characters", max)
	}
}

// Email validates email format
func (v *Validator) Email(field, value string) {
	if value != "" && !ValidateEmail(value) {
		v.errors[field] = "invalid email format"
	}
}

// UUID validates UUID format
func (v *Validator) UUID(field, value string) {
	if value != "" && !ValidateUUID(value) {
		v.errors[field] = "invalid UUID format"
	}
}

// OneOf validates value is in allowed list
func (v *Validator) OneOf(field, value string, allowed []string) {
	if value == "" {
		return
	}
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.errors[field] = fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", "))
}

// Pattern validates against regex pattern
func (v *Validator) Pattern(field, value, pattern, message string) {
	if value == "" {
		return
	}
	matched, err := regexp.MatchString(pattern, value)
	if err != nil || !matched {
		v.errors[field] = message
	}
}

// Valid returns true if no validation errors
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Errors returns validation errors
func (v *Validator) Errors() map[string]string {
	return v.errors
}

// Error returns a formatted error message
func (v *Validator) Error() error {
	if v.Valid() {
		return nil
	}
	var msgs []string
	for field, msg := range v.errors {
		msgs = append(msgs, fmt.Sprintf("%s: %s", field, msg))
	}
	return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
}
