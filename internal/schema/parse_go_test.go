package schema

import (
	"strings"
	"testing"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestParseGoStruct_Simple(t *testing.T) {
	input := `type Response struct {
		Status string ` + "`json:\"status\"`" + `
		Code   int    ` + "`json:\"code\"`" + `
	}`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	} else if codeProp.Type != "integer" {
		t.Errorf("expected code type 'integer', got %q", codeProp.Type)
	}
}

func TestParseGoStruct_Nested(t *testing.T) {
	input := `type Response struct {
		Status string ` + "`json:\"status\"`" + `
		Data   []Item ` + "`json:\"data\"`" + `
	}
	type Item struct {
		ID   int    ` + "`json:\"id\"`" + `
		Name string ` + "`json:\"name\"`" + `
	}`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}

	// Check that Data is an array with ref to Item
	dataProp, ok := schema.Properties["data"]
	if !ok {
		t.Error("expected 'data' property")
	} else {
		if dataProp.Type != "array" {
			t.Errorf("expected data type 'array', got %q", dataProp.Type)
		}
		if dataProp.Items == nil {
			t.Error("expected data.items to be set")
		} else if dataProp.Items.Ref != "#/$defs/Item" {
			t.Errorf("expected data.items.$ref '#/$defs/Item', got %q", dataProp.Items.Ref)
		}
	}

	// Check definitions
	if schema.Definitions == nil {
		t.Error("expected definitions to be set")
	} else {
		itemDef, ok := schema.Definitions["Item"]
		if !ok {
			t.Error("expected 'Item' definition")
		} else if itemDef.Type != "object" {
			t.Errorf("expected Item type 'object', got %q", itemDef.Type)
		}
	}
}

func TestParseGoStruct_Optional(t *testing.T) {
	input := `type Response struct {
		Status string  ` + "`json:\"status\"`" + `
		Meta   *string ` + "`json:\"meta,omitempty\"`" + `
	}`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status should be required (no omitempty, not pointer)
	if len(schema.Required) != 1 || schema.Required[0] != "status" {
		t.Errorf("expected required ['status'], got %v", schema.Required)
	}
}

func TestParseGoStruct_RequiredVsOptional(t *testing.T) {
	input := `type User struct {
		ID       int     ` + "`json:\"id\"`" + `
		Name     string  ` + "`json:\"name\"`" + `
		Email    *string ` + "`json:\"email\"`" + `
		Phone    string  ` + "`json:\"phone,omitempty\"`" + `
		Age      *int    ` + "`json:\"age,omitempty\"`" + `
	}`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only ID and Name should be required
	// - Email is a pointer (optional)
	// - Phone has omitempty (optional)
	// - Age is pointer AND omitempty (optional)
	expectedRequired := map[string]bool{"id": true, "name": true}

	if len(schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d: %v", len(schema.Required), schema.Required)
	}

	for _, r := range schema.Required {
		if !expectedRequired[r] {
			t.Errorf("unexpected required field: %s", r)
		}
	}
}

func TestParseGoStruct_Semicolons(t *testing.T) {
	// Test with semicolons (single-line format)
	input := `type Response struct { Status string ` + "`json:\"status\"`" + `; Code int ` + "`json:\"code\"`" + ` }`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}
}

func TestParseGoStruct_AllTypes(t *testing.T) {
	input := `type AllTypes struct {
		Str     string    ` + "`json:\"str\"`" + `
		Int     int       ` + "`json:\"int\"`" + `
		Int64   int64     ` + "`json:\"int64\"`" + `
		Float   float64   ` + "`json:\"float\"`" + `
		Bool    bool      ` + "`json:\"bool\"`" + `
		Arr     []string  ` + "`json:\"arr\"`" + `
	}`

	schema, err := ParseGoStruct(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"str", "string"},
		{"int", "integer"},
		{"int64", "integer"},
		{"float", "number"},
		{"bool", "boolean"},
		{"arr", "array"},
	}

	for _, tt := range tests {
		prop, ok := schema.Properties[tt.name]
		if !ok {
			t.Errorf("expected '%s' property", tt.name)
			continue
		}
		if prop.Type != tt.expected {
			t.Errorf("expected %s type '%s', got %q", tt.name, tt.expected, prop.Type)
		}
	}
}

func TestParseGoStruct_ForbiddenTypes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name: "any type",
			input: `type Response struct {
				Data any ` + "`json:\"data\"`" + `
			}`,
			expectedErr: "forbidden type \"any\"",
		},
		{
			name: "interface{} type",
			input: `type Response struct {
				Data interface{} ` + "`json:\"data\"`" + `
			}`,
			expectedErr: "forbidden type \"interface{}\"",
		},
		{
			name: "pointer to any",
			input: `type Response struct {
				Data *any ` + "`json:\"data\"`" + `
			}`,
			expectedErr: "forbidden type \"any\"",
		},
		{
			name: "slice of interface{}",
			input: `type Response struct {
				Data []interface{} ` + "`json:\"data\"`" + `
			}`,
			expectedErr: "forbidden type \"interface{}\"",
		},
		{
			name: "map with any value",
			input: `type Response struct {
				Data map[string]any ` + "`json:\"data\"`" + `
			}`,
			expectedErr: "forbidden type \"any\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGoStruct(tt.input)
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				return
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("error = %q, want error containing %q", err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestParseGoStruct_WarningTypes(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedWarning string
	}{
		{
			name: "json.RawMessage type",
			input: `type Response struct {
				Data json.RawMessage ` + "`json:\"data\"`" + `
			}`,
			expectedWarning: "json.RawMessage",
		},
		{
			name: "[]byte type",
			input: `type Response struct {
				Data []byte ` + "`json:\"data\"`" + `
			}`,
			expectedWarning: "[]byte",
		},
		{
			name: "pointer to json.RawMessage",
			input: `type Response struct {
				Data *json.RawMessage ` + "`json:\"data\"`" + `
			}`,
			expectedWarning: "json.RawMessage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseGoStructWithWarnings(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Schema == nil {
				t.Fatal("expected schema, got nil")
			}
			if len(result.Warnings) == 0 {
				t.Errorf("expected warnings containing %q, got none", tt.expectedWarning)
				return
			}
			found := false
			for _, w := range result.Warnings {
				if strings.Contains(w, tt.expectedWarning) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("warnings = %v, want warning containing %q", result.Warnings, tt.expectedWarning)
			}
		})
	}
}

func TestParseGoStruct_MixedTypes(t *testing.T) {
	// Test that valid types with warnings still produce a schema
	input := `type Response struct {
		Status string ` + "`json:\"status\"`" + `
		Raw    json.RawMessage ` + "`json:\"raw\"`" + `
		Code   int ` + "`json:\"code\"`" + `
	}`

	result, err := ParseGoStructWithWarnings(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema == nil {
		t.Fatal("expected schema, got nil")
	}

	if len(result.Schema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(result.Schema.Properties))
	}

	if len(result.Warnings) == 0 {
		t.Error("expected at least one warning for json.RawMessage")
	}
}

func TestParseGoStruct_PointerTypesAllowed(t *testing.T) {
	// Pointer types should be allowed for nullable fields
	input := `type User struct {
		ID        int      ` + "`json:\"id\"`" + `
		Name      *string  ` + "`json:\"name\"`" + `
		Age       *int     ` + "`json:\"age\"`" + `
		Score     *float64 ` + "`json:\"score\"`" + `
		Active    *bool    ` + "`json:\"active\"`" + `
		Tags      *[]string ` + "`json:\"tags\"`" + `
	}`

	result, err := ParseGoStructWithWarnings(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Schema == nil {
		t.Fatal("expected schema, got nil")
	}

	// Should have 6 properties
	if len(result.Schema.Properties) != 6 {
		t.Errorf("expected 6 properties, got %d", len(result.Schema.Properties))
	}

	// Only ID should be required (all pointer types are optional/nullable)
	if len(result.Schema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d: %v", len(result.Schema.Required), result.Schema.Required)
	}
	if len(result.Schema.Required) > 0 && result.Schema.Required[0] != "id" {
		t.Errorf("expected 'id' to be required, got %v", result.Schema.Required)
	}

	// Verify non-pointer type
	idProp := result.Schema.Properties["id"]
	if idProp.Type != "integer" {
		t.Errorf("expected id type 'integer', got %q", idProp.Type)
	}

	// Verify pointer types are nullable (anyOf with null)
	pointerTests := []struct {
		name         string
		expectedType string
	}{
		{"name", "string"},
		{"age", "integer"},
		{"score", "number"},
		{"active", "boolean"},
		{"tags", "array"},
	}

	for _, tt := range pointerTests {
		prop, ok := result.Schema.Properties[tt.name]
		if !ok {
			t.Errorf("expected '%s' property", tt.name)
			continue
		}
		// Pointer types should have anyOf with null
		if len(prop.AnyOf) != 2 {
			t.Errorf("expected %s to have anyOf with 2 options (type + null), got %d", tt.name, len(prop.AnyOf))
			continue
		}
		// First option should be the actual type
		if prop.AnyOf[0].Type != tt.expectedType {
			t.Errorf("expected %s anyOf[0] type '%s', got %q", tt.name, tt.expectedType, prop.AnyOf[0].Type)
		}
		// Second option should be null
		if prop.AnyOf[1].Type != "null" {
			t.Errorf("expected %s anyOf[1] type 'null', got %q", tt.name, prop.AnyOf[1].Type)
		}
	}

	// No warnings should be generated for valid pointer types
	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings for valid pointer types, got: %v", result.Warnings)
	}
}

func TestParseGoStruct_NullableFieldsAcceptNull(t *testing.T) {
	// Test that pointer fields actually accept null values when validated
	input := `type Response struct {
		Status string  ` + "`json:\"status\"`" + `
		Value  *int    ` + "`json:\"value\"`" + `
		Name   *string ` + "`json:\"name\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test with null values for nullable fields - should be valid
	validJSON := `{"status": "ok", "value": null, "name": null}`
	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for null values in nullable fields, got errors: %v", result.Errors)
	}

	// Test with actual values - should also be valid
	validJSON2 := `{"status": "ok", "value": 42, "name": "test"}`
	result2 := validator.Validate([]byte(validJSON2))
	if !result2.Valid {
		t.Errorf("expected valid for actual values, got errors: %v", result2.Errors)
	}

	// Test with omitted nullable fields - should be valid (they're optional)
	validJSON3 := `{"status": "ok"}`
	result3 := validator.Validate([]byte(validJSON3))
	if !result3.Valid {
		t.Errorf("expected valid for omitted nullable fields, got errors: %v", result3.Errors)
	}

	// Test with null for required field - should be invalid
	invalidJSON := `{"status": null, "value": 1}`
	result4 := validator.Validate([]byte(invalidJSON))
	if result4.Valid {
		t.Error("expected invalid for null in required non-pointer field")
	}
}

func TestParseGoStruct_NestedNullableFields(t *testing.T) {
	// Test nested structs with nullable fields
	input := `type Response struct {
		Data   Data   ` + "`json:\"data\"`" + `
		Config *Config ` + "`json:\"config\"`" + `
	}
	type Data struct {
		Value *int    ` + "`json:\"value\"`" + `
		Label *string ` + "`json:\"label\"`" + `
	}
	type Config struct {
		Enabled bool ` + "`json:\"enabled\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test with null nested nullable fields
	validJSON := `{"data": {"value": null, "label": null}, "config": null}`
	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for null nested fields, got errors: %v", result.Errors)
	}

	// Test with actual values
	validJSON2 := `{"data": {"value": 42, "label": "test"}, "config": {"enabled": true}}`
	result2 := validator.Validate([]byte(validJSON2))
	if !result2.Valid {
		t.Errorf("expected valid for actual nested values, got errors: %v", result2.Errors)
	}

	// Test with omitted nullable struct
	validJSON3 := `{"data": {"value": 1}}`
	result3 := validator.Validate([]byte(validJSON3))
	if !result3.Valid {
		t.Errorf("expected valid for omitted nullable struct, got errors: %v", result3.Errors)
	}
}

func TestParseGoStruct_NullableArrayElements(t *testing.T) {
	// Test arrays containing structs with nullable fields
	input := `type Response struct {
		Items []Item ` + "`json:\"items\"`" + `
	}
	type Item struct {
		ID    int     ` + "`json:\"id\"`" + `
		Name  *string ` + "`json:\"name\"`" + `
		Value *int    ` + "`json:\"value\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test with null values in array items
	validJSON := `{"items": [
		{"id": 1, "name": null, "value": null},
		{"id": 2, "name": "test", "value": 42},
		{"id": 3}
	]}`
	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for null values in array items, got errors: %v", result.Errors)
	}
}

func TestParseGoStruct_NullableMapValues(t *testing.T) {
	// Test maps with nullable value types
	input := `type Response struct {
		Data map[string]*int ` + "`json:\"data\"`" + `
	}`

	result, err := ParseGoStructWithWarnings(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Map with pointer values should still work
	if result.Schema == nil {
		t.Fatal("expected schema, got nil")
	}

	dataProp := result.Schema.Properties["data"]
	if dataProp == nil {
		t.Fatal("expected 'data' property")
	}
	if dataProp.Type != "object" {
		t.Errorf("expected data type 'object', got %q", dataProp.Type)
	}
}

func TestParseGoStruct_ComplexRealWorldSchema(t *testing.T) {
	// Test a complex real-world-like schema similar to the Vehicle example
	input := `type ListResponse struct {
		Success bool      ` + "`json:\"success\"`" + `
		Results []Vehicle ` + "`json:\"results\"`" + `
	}
	type Vehicle struct {
		ID          int         ` + "`json:\"id\"`" + `
		Title       string      ` + "`json:\"title\"`" + `
		Engine      Engine      ` + "`json:\"engine\"`" + `
		Consumption Consumption ` + "`json:\"consumption\"`" + `
		Media       Media       ` + "`json:\"media\"`" + `
		RetailerSite RetailerSite ` + "`json:\"retailer_site\"`" + `
	}
	type Engine struct {
		CC     *int     ` + "`json:\"cc\"`" + `
		Litres *float64 ` + "`json:\"litres\"`" + `
	}
	type Consumption struct {
		Energy Energy ` + "`json:\"energy\"`" + `
		Range  Range  ` + "`json:\"range\"`" + `
	}
	type Energy struct {
		Unit  *string ` + "`json:\"unit\"`" + `
		Value *int    ` + "`json:\"value\"`" + `
	}
	type Range struct {
		Unit   *string     ` + "`json:\"unit\"`" + `
		Values RangeValues ` + "`json:\"values\"`" + `
	}
	type RangeValues struct {
		City  *int ` + "`json:\"city\"`" + `
		Total *int ` + "`json:\"total\"`" + `
	}
	type Media struct {
		Items []MediaItem ` + "`json:\"items\"`" + `
		Total int         ` + "`json:\"total\"`" + `
	}
	type MediaItem struct {
		URL          string  ` + "`json:\"url\"`" + `
		DynamicAdURL *string ` + "`json:\"dynamic_ad_url\"`" + `
	}
	type RetailerSite struct {
		Name                string  ` + "`json:\"name\"`" + `
		FinanceDealerNumber *string ` + "`json:\"finance_dealer_number\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test with realistic data including null values
	validJSON := `{
		"success": true,
		"results": [
			{
				"id": 1,
				"title": "Test Vehicle",
				"engine": {"cc": null, "litres": 2.0},
				"consumption": {
					"energy": {"unit": null, "value": null},
					"range": {"unit": "miles", "values": {"city": null, "total": 300}}
				},
				"media": {
					"items": [
						{"url": "http://example.com/img.jpg", "dynamic_ad_url": null},
						{"url": "http://example.com/img2.jpg", "dynamic_ad_url": "http://ad.com"}
					],
					"total": 2
				},
				"retailer_site": {"name": "Dealer", "finance_dealer_number": null}
			}
		]
	}`

	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for complex schema with nulls, got errors: %v", result.Errors)
	}

	// Test with all nullable fields having values
	validJSON2 := `{
		"success": true,
		"results": [
			{
				"id": 1,
				"title": "Test Vehicle",
				"engine": {"cc": 2000, "litres": 2.0},
				"consumption": {
					"energy": {"unit": "kWh", "value": 50},
					"range": {"unit": "miles", "values": {"city": 200, "total": 300}}
				},
				"media": {
					"items": [{"url": "http://example.com/img.jpg", "dynamic_ad_url": "http://ad.com"}],
					"total": 1
				},
				"retailer_site": {"name": "Dealer", "finance_dealer_number": "12345"}
			}
		]
	}`

	result2 := validator.Validate([]byte(validJSON2))
	if !result2.Valid {
		t.Errorf("expected valid for complex schema with all values, got errors: %v", result2.Errors)
	}
}

func TestParseGoStruct_AllNullableTypes(t *testing.T) {
	// Test all primitive types as nullable
	input := `type AllNullable struct {
		NullString  *string  ` + "`json:\"null_string\"`" + `
		NullInt     *int     ` + "`json:\"null_int\"`" + `
		NullInt64   *int64   ` + "`json:\"null_int64\"`" + `
		NullFloat   *float64 ` + "`json:\"null_float\"`" + `
		NullBool    *bool    ` + "`json:\"null_bool\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// All nulls
	validJSON := `{
		"null_string": null,
		"null_int": null,
		"null_int64": null,
		"null_float": null,
		"null_bool": null
	}`
	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for all nulls, got errors: %v", result.Errors)
	}

	// All values
	validJSON2 := `{
		"null_string": "test",
		"null_int": 42,
		"null_int64": 9999999999,
		"null_float": 3.14,
		"null_bool": true
	}`
	result2 := validator.Validate([]byte(validJSON2))
	if !result2.Valid {
		t.Errorf("expected valid for all values, got errors: %v", result2.Errors)
	}

	// Empty object (all fields are optional)
	validJSON3 := `{}`
	result3 := validator.Validate([]byte(validJSON3))
	if !result3.Valid {
		t.Errorf("expected valid for empty object, got errors: %v", result3.Errors)
	}

	// Wrong type should still fail
	invalidJSON := `{"null_int": "not a number"}`
	result4 := validator.Validate([]byte(invalidJSON))
	if result4.Valid {
		t.Error("expected invalid for wrong type in nullable field")
	}
}

func TestParseGoStruct_NullablePointerToStruct(t *testing.T) {
	// Test pointer to struct type
	input := `type Response struct {
		Data    Data   ` + "`json:\"data\"`" + `
		OptData *Data  ` + "`json:\"opt_data\"`" + `
	}
	type Data struct {
		Value int ` + "`json:\"value\"`" + `
	}`

	validator, err := NewValidator(input, types.FormatGoStruct)
	if err != nil {
		t.Fatalf("unexpected error creating validator: %v", err)
	}

	// Test with null for pointer to struct
	validJSON := `{"data": {"value": 1}, "opt_data": null}`
	result := validator.Validate([]byte(validJSON))
	if !result.Valid {
		t.Errorf("expected valid for null pointer to struct, got errors: %v", result.Errors)
	}

	// Test with value for pointer to struct
	validJSON2 := `{"data": {"value": 1}, "opt_data": {"value": 2}}`
	result2 := validator.Validate([]byte(validJSON2))
	if !result2.Valid {
		t.Errorf("expected valid for value in pointer to struct, got errors: %v", result2.Errors)
	}

	// Test with omitted pointer to struct
	validJSON3 := `{"data": {"value": 1}}`
	result3 := validator.Validate([]byte(validJSON3))
	if !result3.Valid {
		t.Errorf("expected valid for omitted pointer to struct, got errors: %v", result3.Errors)
	}
}
