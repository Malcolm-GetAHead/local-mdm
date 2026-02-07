package validation

import (
	"strings"
	"testing"
)

func TestValidateJSONB(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		maxDepth  int
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil data",
			data:      nil,
			maxDepth:  10,
			wantError: false,
		},
		{
			name:      "simple object",
			data:      map[string]interface{}{"key": "value"},
			maxDepth:  10,
			wantError: false,
		},
		{
			name:      "empty object",
			data:      map[string]interface{}{},
			maxDepth:  10,
			wantError: false,
		},
		{
			name:      "empty array",
			data:      []interface{}{},
			maxDepth:  10,
			wantError: false,
		},
		{
			name: "nested object within limit",
			data: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": "value",
					},
				},
			},
			maxDepth:  10,
			wantError: false,
		},
		{
			name: "nested object exceeds depth",
			data: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{
						"l3": "value",
					},
				},
			},
			maxDepth:  2,
			wantError: true,
			errorMsg:  "nesting depth",
		},
		{
			name: "nested object exactly at depth limit",
			data: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": "value",
				},
			},
			maxDepth:  2,
			wantError: false,
		},
		{
			name:      "array within limit",
			data:      []interface{}{"a", "b", "c"},
			maxDepth:  10,
			wantError: false,
		},
		{
			name: "nested array exceeds depth",
			data: []interface{}{
				[]interface{}{
					[]interface{}{"deep"},
				},
			},
			maxDepth:  2,
			wantError: true,
			errorMsg:  "nesting depth",
		},
		{
			name:      "exceeds size limit",
			data:      map[string]interface{}{"data": strings.Repeat("x", MaxJSONBSize)},
			maxDepth:  10,
			wantError: true,
			errorMsg:  "exceeds maximum size",
		},
		{
			name: "at size limit boundary",
			data: map[string]interface{}{"data": strings.Repeat("x", MaxJSONBSize-50)},
			maxDepth: 10,
			wantError: false,
		},
		{
			name: "unmarshalable type",
			data: make(chan int),
			maxDepth: 10,
			wantError: true,
			errorMsg: "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONB(tt.data, tt.maxDepth)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCalculateDepth(t *testing.T) {
	tests := []struct {
		name  string
		data  interface{}
		depth int
	}{
		{
			name:  "primitive",
			data:  "string",
			depth: 0,
		},
		{
			name:  "number",
			data:  42,
			depth: 0,
		},
		{
			name:  "boolean",
			data:  true,
			depth: 0,
		},
		{
			name:  "nil",
			data:  nil,
			depth: 0,
		},
		{
			name:  "empty object",
			data:  map[string]interface{}{},
			depth: 0,
		},
		{
			name:  "empty array",
			data:  []interface{}{},
			depth: 0,
		},
		{
			name:  "flat object",
			data:  map[string]interface{}{"a": 1, "b": 2},
			depth: 1,
		},
		{
			name: "nested object depth 3",
			data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 1,
					},
				},
			},
			depth: 3,
		},
		{
			name:  "flat array",
			data:  []interface{}{1, 2, 3},
			depth: 1,
		},
		{
			name: "nested array depth 3",
			data: []interface{}{
				[]interface{}{
					[]interface{}{1},
				},
			},
			depth: 3,
		},
		{
			name: "mixed nesting",
			data: map[string]interface{}{
				"array": []interface{}{
					map[string]interface{}{
						"nested": "value",
					},
				},
			},
			depth: 3,
		},
		{
			name: "object with multiple branches different depths",
			data: map[string]interface{}{
				"shallow": "value",
				"deep": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": "value",
					},
				},
			},
			depth: 3,
		},
		{
			name: "array with mixed depths",
			data: []interface{}{
				"shallow",
				[]interface{}{
					[]interface{}{"deep"},
				},
			},
			depth: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depth := calculateDepth(tt.data)
			if depth != tt.depth {
				t.Errorf("expected depth %d, got %d", tt.depth, depth)
			}
		})
	}
}
