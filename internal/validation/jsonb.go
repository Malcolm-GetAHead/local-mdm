package validation

import (
	"encoding/json"
	"fmt"
)

const (
	MaxJSONBSize  = 1 << 20 // 1MB
	MaxJSONBDepth = 10
)

// ValidateJSONB validates JSONB data for size and depth
func ValidateJSONB(data interface{}, maxDepth int) error {
	if data == nil {
		return nil
	}

	// For json.RawMessage, check size before parsing (fast path)
	if raw, ok := data.(json.RawMessage); ok {
		if len(raw) > MaxJSONBSize {
			return fmt.Errorf("JSON exceeds maximum size of %d bytes", MaxJSONBSize)
		}

		var obj interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		if depth := calculateDepth(obj); depth > maxDepth {
			return fmt.Errorf("JSON nesting depth %d exceeds maximum of %d", depth, maxDepth)
		}

		return nil
	}

	// For other types, marshal first
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(bytes) > MaxJSONBSize {
		return fmt.Errorf("JSON exceeds maximum size of %d bytes", MaxJSONBSize)
	}

	var obj interface{}
	if err := json.Unmarshal(bytes, &obj); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if depth := calculateDepth(obj); depth > maxDepth {
		return fmt.Errorf("JSON nesting depth %d exceeds maximum of %d", depth, maxDepth)
	}

	return nil
}

func calculateDepth(v interface{}) int {
	switch val := v.(type) {
	case map[string]interface{}:
		if len(val) == 0 {
			return 0
		}
		maxDepth := 0
		for _, item := range val {
			if d := calculateDepth(item); d > maxDepth {
				maxDepth = d
			}
		}
		return maxDepth + 1
	case []interface{}:
		if len(val) == 0 {
			return 0
		}
		maxDepth := 0
		for _, item := range val {
			if d := calculateDepth(item); d > maxDepth {
				maxDepth = d
			}
		}
		return maxDepth + 1
	default:
		return 0
	}
}
