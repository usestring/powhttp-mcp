package schema

import (
	"testing"
)

func TestParseZodSchema_Simple(t *testing.T) {
	input := `z.object({ status: z.string(), code: z.number() })`

	schema, err := ParseZodSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema = PostProcessZodSchema(schema)

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}

	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}

	statusProp, ok := schema.Properties["status"]
	if !ok {
		t.Error("expected 'status' property")
	} else if statusProp.Type != "string" {
		t.Errorf("expected status type 'string', got %q", statusProp.Type)
	}

	codeProp, ok := schema.Properties["code"]
	if !ok {
		t.Error("expected 'code' property")
	} else if codeProp.Type != "number" {
		t.Errorf("expected code type 'number', got %q", codeProp.Type)
	}
}

func TestParseZodSchema_Nested(t *testing.T) {
	input := `z.object({
		status: z.string(),
		data: z.array(z.object({
			id: z.number(),
			name: z.string()
		}))
	})`

	schema, err := ParseZodSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema = PostProcessZodSchema(schema)

	dataProp, ok := schema.Properties["data"]
	if !ok {
		t.Error("expected 'data' property")
	} else {
		if dataProp.Type != "array" {
			t.Errorf("expected data type 'array', got %q", dataProp.Type)
		}
		if dataProp.Items == nil {
			t.Error("expected data.items to be set")
		} else if dataProp.Items.Type != "object" {
			t.Errorf("expected data.items type 'object', got %q", dataProp.Items.Type)
		}
	}
}

func TestParseZodSchema_Optional(t *testing.T) {
	input := `z.object({ status: z.string(), meta: z.string().optional() })`

	schema, err := ParseZodSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema = PostProcessZodSchema(schema)

	// status should be required, meta should not
	foundStatus := false
	foundMeta := false
	for _, r := range schema.Required {
		if r == "status" {
			foundStatus = true
		}
		if r == "meta" {
			foundMeta = true
		}
	}

	if !foundStatus {
		t.Error("expected 'status' to be in required")
	}
	if foundMeta {
		t.Error("expected 'meta' to NOT be in required")
	}
}

func TestParseZodSchema_RequiredVsOptional(t *testing.T) {
	input := `z.object({
		id: z.number(),
		name: z.string(),
		email: z.string().optional(),
		phone: z.string().default(""),
		active: z.boolean()
	})`

	schema, err := ParseZodSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema = PostProcessZodSchema(schema)

	// id, name, active should be required
	// email (optional) and phone (default) should NOT be required
	expectedRequired := map[string]bool{"id": true, "name": true, "active": true}
	expectedOptional := map[string]bool{"email": true, "phone": true}

	if len(schema.Required) != 3 {
		t.Errorf("expected 3 required fields, got %d: %v", len(schema.Required), schema.Required)
	}

	for _, r := range schema.Required {
		if expectedOptional[r] {
			t.Errorf("field %s should be optional, but is required", r)
		}
		if !expectedRequired[r] {
			t.Errorf("unexpected required field: %s", r)
		}
	}
}

func TestParseZodSchema_NestedOptional(t *testing.T) {
	input := `z.object({
		user: z.object({
			id: z.number(),
			name: z.string().optional()
		})
	})`

	schema, err := ParseZodSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schema = PostProcessZodSchema(schema)

	// user should be required at root level
	if len(schema.Required) != 1 || schema.Required[0] != "user" {
		t.Errorf("expected required ['user'], got %v", schema.Required)
	}

	// Check nested object
	userSchema := schema.Properties["user"]
	if userSchema == nil {
		t.Fatal("expected 'user' property")
	}

	// In user, only 'id' should be required (name is optional)
	if len(userSchema.Required) != 1 || userSchema.Required[0] != "id" {
		t.Errorf("expected user.required ['id'], got %v", userSchema.Required)
	}
}

func TestParseZodSchema_AllTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"z.string()", "string"},
		{"z.number()", "number"},
		{"z.boolean()", "boolean"},
		{"z.null()", "null"},
		{"z.array(z.string())", "array"},
	}

	for _, tt := range tests {
		schema, err := ParseZodSchema(tt.input)
		if err != nil {
			t.Errorf("ParseZodSchema(%q) error: %v", tt.input, err)
			continue
		}
		if schema.Type != tt.expected {
			t.Errorf("ParseZodSchema(%q) type = %q, want %q", tt.input, schema.Type, tt.expected)
		}
	}
}

func TestParseZodSchema_ForbiddenTypes(t *testing.T) {
	tests := []struct {
		input       string
		expectedErr string
	}{
		{"z.any()", "z.any() is forbidden"},
		{"z.unknown()", "z.unknown() is forbidden"},
		{"z.object({ data: z.any() })", "z.any() is forbidden"},
		{"z.array(z.unknown())", "z.unknown() is forbidden"},
	}

	for _, tt := range tests {
		_, err := ParseZodSchema(tt.input)
		if err == nil {
			t.Errorf("ParseZodSchema(%q) expected error containing %q, got nil", tt.input, tt.expectedErr)
			continue
		}
		if !contains(err.Error(), tt.expectedErr) {
			t.Errorf("ParseZodSchema(%q) error = %q, want error containing %q", tt.input, err.Error(), tt.expectedErr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
