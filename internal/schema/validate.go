package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// Validator validates JSON data against a schema.
type Validator struct {
	schema   *jsonschema.Schema
	compiled bool
}

// NewValidator creates a new validator from a schema definition.
func NewValidator(schemaStr string, format types.SchemaFormat) (*Validator, error) {
	var jsonSchema *JSONSchema
	var err error

	switch format {
	case types.FormatGoStruct:
		jsonSchema, err = ParseGoStruct(schemaStr)
		if err != nil {
			return nil, fmt.Errorf("parsing Go struct: %w", err)
		}

	case types.FormatZod:
		jsonSchema, err = ParseZodSchema(schemaStr)
		if err != nil {
			return nil, fmt.Errorf("parsing Zod schema: %w", err)
		}
		// Post-process to handle optional markers
		jsonSchema = PostProcessZodSchema(jsonSchema)

	case types.FormatJSONSchema:
		// Parse as raw JSON Schema
		if err := json.Unmarshal([]byte(schemaStr), &jsonSchema); err != nil {
			return nil, fmt.Errorf("parsing JSON Schema: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown schema format: %s", format)
	}

	// Convert our JSONSchema to the validator's format
	return compileSchema(jsonSchema)
}

// NewValidatorFromJSONSchema creates a validator from a pre-parsed JSONSchema.
func NewValidatorFromJSONSchema(schema *JSONSchema) (*Validator, error) {
	return compileSchema(schema)
}

// compileSchema compiles a JSONSchema into a validator.
func compileSchema(schema *JSONSchema) (*Validator, error) {
	// Convert to JSON and back to get a clean map[string]any
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshaling schema: %w", err)
	}

	var schemaValue any
	if err := json.Unmarshal(schemaJSON, &schemaValue); err != nil {
		return nil, fmt.Errorf("unmarshaling schema: %w", err)
	}

	// Create a compiler
	compiler := jsonschema.NewCompiler()

	// Add the schema as a resource (doc must be valid json value, not io.Reader)
	if err := compiler.AddResource("schema.json", schemaValue); err != nil {
		return nil, fmt.Errorf("adding schema resource: %w", err)
	}

	// Compile the schema
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compiling schema: %w", err)
	}

	return &Validator{
		schema:   compiled,
		compiled: true,
	}, nil
}

// Validate validates a JSON value against the schema.
func (v *Validator) Validate(data []byte) *types.ValidationResult {
	if !v.compiled || v.schema == nil {
		return &types.ValidationResult{
			Valid:  false,
			Errors: []string{"schema not compiled"},
		}
	}

	// Parse the JSON
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return &types.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid JSON: %s", err.Error())},
		}
	}

	// Validate
	err := v.schema.Validate(value)
	if err == nil {
		return &types.ValidationResult{Valid: true}
	}

	// Extract validation errors
	errors := extractValidationErrors(err)
	return &types.ValidationResult{
		Valid:  false,
		Errors: errors,
	}
}

// ValidateValue validates an already-parsed value against the schema.
func (v *Validator) ValidateValue(value any) *types.ValidationResult {
	if !v.compiled || v.schema == nil {
		return &types.ValidationResult{
			Valid:  false,
			Errors: []string{"schema not compiled"},
		}
	}

	err := v.schema.Validate(value)
	if err == nil {
		return &types.ValidationResult{Valid: true}
	}

	errors := extractValidationErrors(err)
	return &types.ValidationResult{
		Valid:  false,
		Errors: errors,
	}
}

// extractValidationErrors extracts human-readable error messages from a validation error.
func extractValidationErrors(err error) []string {
	if err == nil {
		return nil
	}

	// Try to get detailed errors from the jsonschema library
	var validationErr *jsonschema.ValidationError
	if errors.As(err, &validationErr) {
		return extractDetailedErrors(validationErr)
	}

	// Fallback to simple error message
	return []string{err.Error()}
}

// printer is a default English printer for localized error messages.
var printer = message.NewPrinter(language.English)

// extractDetailedErrors recursively extracts errors from a ValidationError.
func extractDetailedErrors(err *jsonschema.ValidationError) []string {
	// Use a map to deduplicate and aggregate errors by path
	errorsByPath := make(map[string][]string)
	collectErrors(err, errorsByPath)

	// Build a clean, deduplicated list
	var result []string
	for path, msgs := range errorsByPath {
		// Deduplicate messages for this path
		seen := make(map[string]bool)
		for _, msg := range msgs {
			if !seen[msg] {
				seen[msg] = true
				if path != "" {
					result = append(result, fmt.Sprintf("%s: %s", path, msg))
				} else {
					result = append(result, msg)
				}
			}
		}
	}

	return result
}

// collectErrors recursively collects leaf errors (those without causes).
func collectErrors(err *jsonschema.ValidationError, errorsByPath map[string][]string) {
	// Build the instance location path
	instancePath := ""
	if len(err.InstanceLocation) > 0 {
		instancePath = "/" + strings.Join(err.InstanceLocation, "/")
	}

	// Only collect leaf errors (those with actual error kinds and no causes)
	// or errors that have both an error kind and represent a concrete validation failure
	if err.ErrorKind != nil && len(err.Causes) == 0 {
		// Use LocalizedString for proper human-readable error message
		errMsg := err.ErrorKind.LocalizedString(printer)
		// Skip $ref and schema reference messages - they're not useful errors
		if !strings.HasPrefix(errMsg, "$ref ") && !strings.HasPrefix(errMsg, "doesn't validate with") {
			errorsByPath[instancePath] = append(errorsByPath[instancePath], errMsg)
		}
	}

	// Recurse into causes
	for _, cause := range err.Causes {
		collectErrors(cause, errorsByPath)
	}
}

// GetSchema returns the JSON Schema used by this validator.
func (v *Validator) GetSchema() *jsonschema.Schema {
	return v.schema
}

// SchemaToMap converts a JSONSchema to a map for JSON serialization.
func SchemaToMap(schema *JSONSchema) (map[string]any, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}
