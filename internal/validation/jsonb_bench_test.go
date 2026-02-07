package validation

import (
	"strings"
	"testing"
)

func BenchmarkValidateJSONB_Small(b *testing.B) {
	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateJSONB(data, MaxJSONBDepth)
	}
}

func BenchmarkValidateJSONB_Medium(b *testing.B) {
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"profile": map[string]interface{}{
				"age":     30,
				"city":    "New York",
				"country": "USA",
			},
		},
		"settings": map[string]interface{}{
			"theme":         "dark",
			"notifications": true,
			"language":      "en",
		},
		"data": strings.Repeat("x", 10000),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateJSONB(data, MaxJSONBDepth)
	}
}

func BenchmarkValidateJSONB_Large(b *testing.B) {
	data := map[string]interface{}{
		"data": strings.Repeat("x", 500000),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateJSONB(data, MaxJSONBDepth)
	}
}

func BenchmarkValidateJSONB_DeepNesting(b *testing.B) {
	data := map[string]interface{}{
		"l1": map[string]interface{}{
			"l2": map[string]interface{}{
				"l3": map[string]interface{}{
					"l4": map[string]interface{}{
						"l5": map[string]interface{}{
							"l6": map[string]interface{}{
								"l7": map[string]interface{}{
									"l8": "value",
								},
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateJSONB(data, MaxJSONBDepth)
	}
}

func BenchmarkCalculateDepth_Flat(b *testing.B) {
	data := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateDepth(data)
	}
}

func BenchmarkCalculateDepth_Nested(b *testing.B) {
	data := map[string]interface{}{
		"l1": map[string]interface{}{
			"l2": map[string]interface{}{
				"l3": map[string]interface{}{
					"l4": map[string]interface{}{
						"l5": "value",
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateDepth(data)
	}
}

func BenchmarkCalculateDepth_Array(b *testing.B) {
	data := []interface{}{
		[]interface{}{
			[]interface{}{
				[]interface{}{
					"value",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateDepth(data)
	}
}
