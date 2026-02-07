package models

import (
	"database/sql/driver"
	"testing"
)

func TestJSONB_Value(t *testing.T) {
	tests := []struct {
		name      string
		jsonb     JSONB
		wantNil   bool
		wantError bool
	}{
		{
			name:    "nil JSONB",
			jsonb:   nil,
			wantNil: true,
		},
		{
			name:    "empty JSONB",
			jsonb:   JSONB{},
			wantNil: false,
		},
		{
			name: "simple JSONB",
			jsonb: JSONB{
				"key": "value",
			},
			wantNil: false,
		},
		{
			name: "nested JSONB",
			jsonb: JSONB{
				"user": map[string]interface{}{
					"name": "test",
					"age":  30,
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.jsonb.Value()
			
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if tt.wantNil {
				if val != nil {
					t.Errorf("expected nil value, got %v", val)
				}
			} else {
				if val == nil {
					t.Error("expected non-nil value, got nil")
				}
			}
		})
	}
}

func TestJSONB_Scan(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantNil   bool
		wantError bool
	}{
		{
			name:    "nil input",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "empty JSON bytes",
			input:   []byte("{}"),
			wantNil: false,
		},
		{
			name:    "valid JSON bytes",
			input:   []byte(`{"key":"value"}`),
			wantNil: false,
		},
		{
			name:    "nested JSON bytes",
			input:   []byte(`{"user":{"name":"test","age":30}}`),
			wantNil: false,
		},
		{
			name:      "invalid JSON bytes",
			input:     []byte(`{invalid}`),
			wantError: true,
		},
		{
			name:    "non-byte input",
			input:   "string",
			wantNil: false,
		},
		{
			name:    "int input",
			input:   123,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSONB
			err := j.Scan(tt.input)
			
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if tt.wantNil {
				if j != nil {
					t.Errorf("expected nil JSONB, got %v", j)
				}
			} else {
				// For non-byte inputs, JSONB should remain unchanged (nil)
				if _, ok := tt.input.([]byte); !ok {
					if j != nil {
						t.Errorf("expected nil JSONB for non-byte input, got %v", j)
					}
				}
			}
		})
	}
}

func TestJSONB_RoundTrip(t *testing.T) {
	original := JSONB{
		"string": "value",
		"number": float64(42),
		"bool":   true,
		"nested": map[string]interface{}{
			"key": "value",
		},
		"array": []interface{}{1, 2, 3},
	}

	// Convert to driver.Value
	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value() failed: %v", err)
	}

	// Convert back via Scan
	var result JSONB
	if err := result.Scan(val); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	// Verify all keys exist
	if result["string"] != "value" {
		t.Errorf("string mismatch: got %v", result["string"])
	}
	if result["number"] != float64(42) {
		t.Errorf("number mismatch: got %v", result["number"])
	}
	if result["bool"] != true {
		t.Errorf("bool mismatch: got %v", result["bool"])
	}
	if result["nested"] == nil {
		t.Error("nested is nil")
	}
	if result["array"] == nil {
		t.Error("array is nil")
	}
}

func TestJSONB_DriverValuer(t *testing.T) {
	var _ driver.Valuer = JSONB{}
}
