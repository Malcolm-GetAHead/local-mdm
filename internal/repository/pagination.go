package repository

import (
	"context"
	"database/sql"
	"fmt"
)

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

// ExecutePaginatedQuery executes a count query and a data query with pagination
// Returns the results, total count, and any error
func ExecutePaginatedQuery[T any](
	ctx context.Context,
	exec executor,
	countQuery string,
	countArgs []interface{},
	dataQuery string,
	dataArgs []interface{},
	scanFn func(*sql.Rows) (T, error),
) ([]T, int, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// Get total count
	var total int
	if err := exec.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Check context before data query
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// Execute data query
	rows, err := exec.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("data query failed: %w", err)
	}
	defer rows.Close()

	// Scan results
	results := []T{}
	for rows.Next() {
		// Check context during iteration
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}

		item, err := scanFn(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	return results, total, nil
}
