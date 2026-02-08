package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePagination(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		offset        int
		wantLimit     int
		wantOffset    int
		wantErr       bool
		errContains   string
	}{
		{
			name:       "valid pagination",
			limit:      50,
			offset:     10,
			wantLimit:  50,
			wantOffset: 10,
			wantErr:    false,
		},
		{
			name:       "zero limit defaults to 100",
			limit:      0,
			offset:     0,
			wantLimit:  DefaultPageSize,
			wantOffset: 0,
			wantErr:    false,
		},
		{
			name:       "negative limit defaults to 100",
			limit:      -1,
			offset:     0,
			wantLimit:  DefaultPageSize,
			wantOffset: 0,
			wantErr:    false,
		},
		{
			name:        "limit exceeds maximum",
			limit:       10000,
			offset:      0,
			wantErr:     true,
			errContains: "exceeds maximum",
		},
		{
			name:        "limit at maximum allowed",
			limit:       MaxPageSize,
			offset:      0,
			wantLimit:   MaxPageSize,
			wantOffset:  0,
			wantErr:     false,
		},
		{
			name:        "negative offset",
			limit:       100,
			offset:      -1,
			wantErr:     true,
			errContains: "must be non-negative",
		},
		{
			name:       "large valid offset",
			limit:      100,
			offset:     10000,
			wantLimit:  100,
			wantOffset: 10000,
			wantErr:    false,
		},
		{
			name:       "limit of 1",
			limit:      1,
			offset:     0,
			wantLimit:  1,
			wantOffset: 0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotOffset, err := ValidatePagination(tt.limit, tt.offset)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantLimit, gotLimit)
				assert.Equal(t, tt.wantOffset, gotOffset)
			}
		})
	}
}

func TestValidatePagination_EdgeCases(t *testing.T) {
	t.Run("maximum limit boundary", func(t *testing.T) {
		limit, offset, err := ValidatePagination(MaxPageSize, 0)
		require.NoError(t, err)
		assert.Equal(t, MaxPageSize, limit)
		assert.Equal(t, 0, offset)
	})

	t.Run("maximum limit plus one", func(t *testing.T) {
		_, _, err := ValidatePagination(MaxPageSize+1, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("zero offset is valid", func(t *testing.T) {
		limit, offset, err := ValidatePagination(100, 0)
		require.NoError(t, err)
		assert.Equal(t, 100, limit)
		assert.Equal(t, 0, offset)
	})
}
