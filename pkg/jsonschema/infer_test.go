package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestInfer_PrimitiveTypes(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{"string", `"hello"`, "string"},
		{"integer", `42`, "integer"},
		{"float", `3.14`, "number"},
		{"boolean_true", `true`, "boolean"},
		{"boolean_false", `false`, "boolean"},
		{"null", `null`, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Infer([]byte(tt.json))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Schema.Type != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, result.Schema.Type)
			}
		})
	}
}

func TestInfer_IntegerDetection(t *testing.T) {
	// JSON numbers that are whole numbers should be detected as integers
	tests := []struct {
		json     string
		expected string
	}{
		{`0`, "integer"},
		{`1`, "integer"},
		{`-1`, "integer"},
		{`100`, "integer"},
		{`1000000`, "integer"},
		{`1.0`, "integer"}, // 1.0 is a whole number
		{`1.5`, "number"},
		{`0.1`, "number"},
		{`-3.14`, "number"},
	}

	for _, tt := range tests {
		t.Run(tt.json, func(t *testing.T) {
			result, err := Infer([]byte(tt.json))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Schema.Type != tt.expected {
				t.Errorf("Infer(%s) type = %q, want %q", tt.json, result.Schema.Type, tt.expected)
			}
		})
	}
}

func TestInfer_Object(t *testing.T) {
	jsonData := `{"name": "Alice", "age": 30, "active": true}`

	result, err := Infer([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", result.Schema.Type)
	}

	if result.Schema.Properties == nil {
		t.Fatal("expected properties to be set")
	}

	// Check properties
	nameSchema := result.Schema.Properties.GetPair("name")
	if nameSchema == nil || nameSchema.Value.Type != "string" {
		t.Errorf("expected 'name' property with type 'string'")
	}

	ageSchema := result.Schema.Properties.GetPair("age")
	if ageSchema == nil || ageSchema.Value.Type != "integer" {
		t.Errorf("expected 'age' property with type 'integer'")
	}

	activeSchema := result.Schema.Properties.GetPair("active")
	if activeSchema == nil || activeSchema.Value.Type != "boolean" {
		t.Errorf("expected 'active' property with type 'boolean'")
	}
}

func TestInfer_NestedObject(t *testing.T) {
	jsonData := `{
		"user": {
			"id": 1,
			"profile": {
				"name": "Bob"
			}
		}
	}`

	result, err := Infer([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", result.Schema.Type)
	}

	userPair := result.Schema.Properties.GetPair("user")
	if userPair == nil {
		t.Fatal("expected 'user' property")
	}

	userSchema := userPair.Value
	if userSchema.Type != "object" {
		t.Errorf("expected user type 'object', got %q", userSchema.Type)
	}

	profilePair := userSchema.Properties.GetPair("profile")
	if profilePair == nil {
		t.Fatal("expected 'profile' property")
	}

	profileSchema := profilePair.Value
	if profileSchema.Type != "object" {
		t.Errorf("expected profile type 'object', got %q", profileSchema.Type)
	}
}

func TestInfer_Array(t *testing.T) {
	jsonData := `[1, 2, 3]`

	result, err := Infer([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema.Type != "array" {
		t.Errorf("expected type 'array', got %q", result.Schema.Type)
	}

	if result.Schema.Items == nil {
		t.Fatal("expected items schema")
	}

	if result.Schema.Items.Type != "integer" {
		t.Errorf("expected items type 'integer', got %q", result.Schema.Items.Type)
	}
}

func TestInfer_ArrayOfObjects(t *testing.T) {
	jsonData := `[{"id": 1, "name": "A"}, {"id": 2, "name": "B"}]`

	result, err := Infer([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema.Type != "array" {
		t.Errorf("expected type 'array', got %q", result.Schema.Type)
	}

	if result.Schema.Items == nil {
		t.Fatal("expected items schema")
	}

	if result.Schema.Items.Type != "object" {
		t.Errorf("expected items type 'object', got %q", result.Schema.Items.Type)
	}
}

func TestInfer_MergeMultipleSamples(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "name": "Bob"}`),
		[]byte(`{"id": 3, "name": "Charlie"}`),
	}

	result, err := Infer(samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SampleCount != 3 {
		t.Errorf("expected sample count 3, got %d", result.SampleCount)
	}

	if !result.AllMatch {
		t.Error("expected AllMatch to be true for identical schemas")
	}
}

func TestInfer_MergeDifferentSchemas(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "age": 30}`), // Different property
	}

	result, err := Infer(samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AllMatch {
		t.Error("expected AllMatch to be false for different schemas")
	}

	// Should have merged properties from both
	if result.Schema.Properties == nil {
		t.Fatal("expected properties")
	}

	idPair := result.Schema.Properties.GetPair("id")
	namePair := result.Schema.Properties.GetPair("name")
	agePair := result.Schema.Properties.GetPair("age")

	if idPair == nil {
		t.Error("expected 'id' property in merged schema")
	}
	if namePair == nil {
		t.Error("expected 'name' property in merged schema")
	}
	if agePair == nil {
		t.Error("expected 'age' property in merged schema")
	}
}

func TestInferWithOptions_Required(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "name": "Bob"}`),
		[]byte(`{"id": 3}`), // Missing 'name'
	}

	opts := &InferOptions{
		StrictRequired: true,
	}

	result, err := InferWithOptions(opts, samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 'id' should be required (present in all samples)
	if len(result.Schema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(result.Schema.Required))
	}

	if len(result.Schema.Required) > 0 && result.Schema.Required[0] != "id" {
		t.Errorf("expected required[0] = 'id', got %q", result.Schema.Required[0])
	}
}

func TestInferWithOptions_NullableOptional(t *testing.T) {
	// All samples have 'id' and 'name', but 'name' is sometimes null
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "name": null}`), // name is null
		[]byte(`{"id": 3, "name": "Charlie"}`),
	}

	opts := &InferOptions{
		StrictRequired:         true,
		MarkNullableAsOptional: true, // Fields with null values are optional
	}

	result, err := InferWithOptions(opts, samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 'id' should be required (name is sometimes null)
	if len(result.Schema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d: %v", len(result.Schema.Required), result.Schema.Required)
	}

	if len(result.Schema.Required) > 0 && result.Schema.Required[0] != "id" {
		t.Errorf("expected required[0] = 'id', got %q", result.Schema.Required[0])
	}
}

func TestInferWithOptions_NullableNotOptional(t *testing.T) {
	// All samples have 'id' and 'name', but 'name' is sometimes null
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2, "name": null}`), // name is null
		[]byte(`{"id": 3, "name": "Charlie"}`),
	}

	opts := &InferOptions{
		StrictRequired:         true,
		MarkNullableAsOptional: false, // Fields with null values ARE required
	}

	result, err := InferWithOptions(opts, samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both 'id' and 'name' should be required (present in all samples, nullable doesn't matter)
	if len(result.Schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d: %v", len(result.Schema.Required), result.Schema.Required)
	}
}

func TestInfer_SingleSampleAllRequired(t *testing.T) {
	// With a single sample, all present fields should be required
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice", "active": true}`),
	}

	result, err := Infer(samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 fields should be required
	if len(result.Schema.Required) != 3 {
		t.Errorf("expected 3 required fields for single sample, got %d: %v", len(result.Schema.Required), result.Schema.Required)
	}
}

func TestInferWithOptions_AdditionalProperties(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
	}

	falseVal := false
	opts := &InferOptions{
		StrictRequired:       false,
		AdditionalProperties: &falseVal,
	}

	result, err := InferWithOptions(opts, samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Schema should have additionalProperties set
	if result.Schema.AdditionalProperties == nil {
		t.Error("expected AdditionalProperties to be set")
	}
}

func TestInfer_TypeUnion(t *testing.T) {
	// Array with mixed types
	samples := [][]byte{
		[]byte(`"hello"`),
		[]byte(`42`),
	}

	result, err := Infer(samples...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create anyOf for type union
	if result.Schema.AnyOf == nil {
		t.Error("expected anyOf for type union")
	}
}

func TestInfer_Empty(t *testing.T) {
	result, err := Infer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty input")
	}
}

func TestInfer_InvalidJSON(t *testing.T) {
	result, err := Infer([]byte(`not valid json`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid JSON is skipped, so result should be nil
	if result != nil {
		t.Error("expected nil result for invalid JSON")
	}
}

func TestInferFromValue(t *testing.T) {
	value := map[string]any{
		"id":   float64(1),
		"name": "test",
	}

	schema := InferFromValue(value)

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}
}

func TestInfer_DeepNesting(t *testing.T) {
	jsonData := `{
		"level1": {
			"level2": {
				"level3": {
					"value": "deep"
				}
			}
		}
	}`

	result, err := Infer([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we can marshal it back
	_, err = json.Marshal(result.Schema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}
}
