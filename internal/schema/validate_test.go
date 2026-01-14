package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestValidator_JSONSchema(t *testing.T) {
	schemaStr := `{"type": "object", "properties": {"name": {"type": "string"}, "age": {"type": "integer"}}, "required": ["name"]}`

	validator, err := NewValidator(schemaStr, types.FormatJSONSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Valid data
	validData := []byte(`{"name": "Alice", "age": 30}`)
	result := validator.Validate(validData)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}

	// Invalid data - missing required field
	invalidData := []byte(`{"age": 30}`)
	result = validator.Validate(invalidData)
	if result.Valid {
		t.Error("expected invalid for missing required field")
	}

	// Invalid data - wrong type
	invalidData2 := []byte(`{"name": "Alice", "age": "thirty"}`)
	result = validator.Validate(invalidData2)
	if result.Valid {
		t.Error("expected invalid for wrong type")
	}
}

func TestValidator_GoStruct(t *testing.T) {
	schemaStr := `type User struct {
		Name string ` + "`json:\"name\"`" + `
		Age  int    ` + "`json:\"age\"`" + `
	}`

	validator, err := NewValidator(schemaStr, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Valid data
	validData := []byte(`{"name": "Bob", "age": 25}`)
	result := validator.Validate(validData)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}

	// Invalid data - wrong type
	invalidData := []byte(`{"name": 123, "age": 25}`)
	result = validator.Validate(invalidData)
	if result.Valid {
		t.Error("expected invalid for wrong type")
	}
}

func TestValidator_Zod(t *testing.T) {
	schemaStr := `z.object({ name: z.string(), age: z.number() })`

	validator, err := NewValidator(schemaStr, types.FormatZod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Valid data
	validData := []byte(`{"name": "Charlie", "age": 35}`)
	result := validator.Validate(validData)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}

	// Invalid data
	invalidData := []byte(`{"name": "Charlie", "age": "old"}`)
	result = validator.Validate(invalidData)
	if result.Valid {
		t.Error("expected invalid for wrong type")
	}
}

func TestSchemaToMap(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	m, err := SchemaToMap(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the map structure
	if m["type"] != "object" {
		t.Errorf("expected type 'object', got %v", m["type"])
	}

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Error("expected properties to be a map")
	} else if len(props) != 2 {
		t.Errorf("expected 2 properties, got %d", len(props))
	}
}

func TestJSONSchemaMarshaling(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"id":   {Type: "integer"},
			"name": {Type: "string"},
		},
		Required: []string{"id", "name"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	var unmarshaled JSONSchema
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal schema: %v", err)
	}

	if unmarshaled.Type != schema.Type {
		t.Errorf("type mismatch: got %q, want %q", unmarshaled.Type, schema.Type)
	}

	if len(unmarshaled.Properties) != len(schema.Properties) {
		t.Errorf("properties count mismatch: got %d, want %d", len(unmarshaled.Properties), len(schema.Properties))
	}

	if len(unmarshaled.Required) != len(schema.Required) {
		t.Errorf("required count mismatch: got %d, want %d", len(unmarshaled.Required), len(schema.Required))
	}
}

func TestValidationErrorMessages_HumanReadable(t *testing.T) {
	tests := []struct {
		name           string
		schema         string
		data           string
		wantContains   []string // error messages should contain these
		wantNotContain []string // error messages should NOT contain these (raw Go structs)
	}{
		{
			name:   "type mismatch - string expected",
			schema: `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`,
			data:   `{"name": 123}`,
			wantContains: []string{
				"string",
			},
			wantNotContain: []string{
				"&{", // raw Go struct
				"file:///",
				"$ref",
			},
		},
		{
			name:   "type mismatch - integer expected",
			schema: `{"type": "object", "properties": {"age": {"type": "integer"}}, "required": ["age"]}`,
			data:   `{"age": "twenty"}`,
			wantContains: []string{
				"integer",
			},
			wantNotContain: []string{
				"&{",
				"file:///",
			},
		},
		{
			name:   "missing required property",
			schema: `{"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}}, "required": ["id", "name"]}`,
			data:   `{"id": 1}`,
			wantContains: []string{
				"name",
			},
			wantNotContain: []string{
				"&{",
				"file:///",
			},
		},
		{
			name:   "null where integer expected",
			schema: `{"type": "object", "properties": {"count": {"type": "integer"}}, "required": ["count"]}`,
			data:   `{"count": null}`,
			wantContains: []string{
				"integer",
			},
			wantNotContain: []string{
				"&{null [integer]}",
				"&{",
			},
		},
		{
			name: "nested object validation",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"properties": {
							"email": {"type": "string"}
						},
						"required": ["email"]
					}
				},
				"required": ["user"]
			}`,
			data: `{"user": {"email": 12345}}`,
			wantContains: []string{
				"string",
			},
			wantNotContain: []string{
				"&{",
				"file:///",
				"$defs",
			},
		},
		{
			name: "array item validation",
			schema: `{
				"type": "object",
				"properties": {
					"items": {
						"type": "array",
						"items": {"type": "integer"}
					}
				},
				"required": ["items"]
			}`,
			data: `{"items": [1, 2, "three", 4]}`,
			wantContains: []string{
				"/items/2",
				"integer",
			},
			wantNotContain: []string{
				"&{",
				"file:///",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(tt.schema, types.FormatJSONSchema)
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			result := validator.Validate([]byte(tt.data))
			if result.Valid {
				t.Fatal("expected validation to fail")
			}

			if len(result.Errors) == 0 {
				t.Fatal("expected at least one error message")
			}

			// Check that expected substrings are present
			for _, want := range tt.wantContains {
				found := false
				for _, e := range result.Errors {
					if strings.Contains(e, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error to contain %q, got errors: %v", want, result.Errors)
				}
			}

			// Check that unwanted substrings are NOT present
			for _, notWant := range tt.wantNotContain {
				for _, e := range result.Errors {
					if strings.Contains(e, notWant) {
						t.Errorf("error message should not contain %q, got: %q", notWant, e)
					}
				}
			}

			// Log the actual errors for debugging
			t.Logf("Validation errors: %v", result.Errors)
		})
	}
}
