package repository

import "fmt"

const (
	MaxPageSize     = 1000
	DefaultPageSize = 100
)

// ValidatePagination validates and normalizes pagination parameters
func ValidatePagination(limit, offset int) (int, int, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}

	if limit > MaxPageSize {
		return 0, 0, fmt.Errorf("limit exceeds maximum of %d", MaxPageSize)
	}

	if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be non-negative")
	}

	return limit, offset, nil
}
