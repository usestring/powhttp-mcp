// Package schema provides schema parsing and validation utilities for HTTP body validation.
package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseResult contains the parsed schema and any warnings encountered.
type ParseResult struct {
	Schema   *JSONSchema
	Warnings []string
}

// GoStructParser parses Go struct definitions into JSON Schema.
type GoStructParser struct {
	// structs holds all parsed struct definitions keyed by name
	structs map[string]*JSONSchema
	// warnings collects non-fatal issues found during parsing
	warnings []string
}

// JSONSchema represents a JSON Schema object.
// This is a simplified representation that can be marshaled to standard JSON Schema.
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	Ref                  string                 `json:"$ref,omitempty"`
	Definitions          map[string]*JSONSchema `json:"$defs,omitempty"`
}

// ParseGoStruct parses one or more Go struct definitions and returns a JSON Schema.
// The first struct defined is treated as the root schema.
// Nested structs are supported through $defs references.
//
// Returns an error if the schema contains forbidden types (any, interface{}).
// Returns warnings for types that allow arbitrary data (json.RawMessage, []byte).
//
// Example input:
//
//	type Response struct {
//	    Status string `json:"status"`
//	    Data   []Item `json:"data"`
//	}
//	type Item struct {
//	    ID   int    `json:"id"`
//	    Name string `json:"name,omitempty"`
//	}
func ParseGoStruct(input string) (*JSONSchema, error) {
	result, err := ParseGoStructWithWarnings(input)
	if err != nil {
		return nil, err
	}
	return result.Schema, nil
}

// ParseGoStructWithWarnings parses Go struct definitions and returns the schema along with warnings.
// Returns an error if the schema contains forbidden types (any, interface{}).
// Warnings are returned for types that allow arbitrary data (json.RawMessage, []byte).
func ParseGoStructWithWarnings(input string) (*ParseResult, error) {
	parser := &GoStructParser{
		structs:  make(map[string]*JSONSchema),
		warnings: make([]string, 0),
	}

	// Extract all struct definitions
	structDefs := extractStructDefs(input)
	if len(structDefs) == 0 {
		return nil, fmt.Errorf("no struct definitions found in input")
	}

	var rootName string

	// First pass: register all struct names
	for i, def := range structDefs {
		name := def.name
		if i == 0 {
			rootName = name
		}
		parser.structs[name] = nil // placeholder
	}

	// Second pass: parse struct bodies
	for _, def := range structDefs {
		schema, err := parser.parseStructBody(def.name, def.body)
		if err != nil {
			return nil, fmt.Errorf("parsing struct %s: %w", def.name, err)
		}
		parser.structs[def.name] = schema
	}

	// Third pass: resolve references and build final schema
	rootSchema := parser.structs[rootName]
	if rootSchema == nil {
		return nil, fmt.Errorf("root struct %s not found", rootName)
	}

	// Add definitions for other structs if they exist
	if len(parser.structs) > 1 {
		rootSchema.Definitions = make(map[string]*JSONSchema)
		for name, schema := range parser.structs {
			if name != rootName && schema != nil {
				rootSchema.Definitions[name] = schema
			}
		}
	}

	return &ParseResult{
		Schema:   rootSchema,
		Warnings: parser.warnings,
	}, nil
}

// structDef holds a parsed struct definition.
type structDef struct {
	name string
	body string
}

// extractStructDefs extracts all struct definitions from the input.
// Uses a regex to find struct headers, then manually extracts bodies to handle nested braces.
var structHeaderRegex = regexp.MustCompile(`type\s+(\w+)\s+struct\s*\{`)

func extractStructDefs(input string) []structDef {
	// Normalize line endings and handle semicolon-separated fields
	input = strings.ReplaceAll(input, "\\n", "\n")
	input = strings.ReplaceAll(input, ";", "\n")

	defs := make([]structDef, 0)

	// Find all struct headers
	matches := structHeaderRegex.FindAllStringSubmatchIndex(input, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := input[match[2]:match[3]]
			// match[1] is where the opening brace ends
			bodyStart := match[1]

			// Find the matching closing brace, accounting for nested braces
			body, ok := extractBalancedBraces(input[bodyStart-1:])
			if ok && len(body) >= 2 {
				// Remove the outer braces
				innerBody := body[1 : len(body)-1]
				defs = append(defs, structDef{
					name: name,
					body: innerBody,
				})
			}
		}
	}

	return defs
}

// extractBalancedBraces extracts a brace-balanced substring starting with '{'.
// Returns the substring including the braces, and a success flag.
func extractBalancedBraces(s string) (string, bool) {
	if len(s) == 0 || s[0] != '{' {
		return "", false
	}

	depth := 0
	inString := false
	inRawString := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		// Handle string literals (skip contents)
		if inString {
			if ch == '\\' && i+1 < len(s) {
				i++ // Skip escaped character
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inRawString {
			if ch == '`' {
				inRawString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '`':
			inRawString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}

	return "", false
}

// parseStructBody parses the body of a struct definition.
func (p *GoStructParser) parseStructBody(structName, body string) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
		Required:   make([]string, 0),
	}

	// Parse each field
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		field, err := p.parseField(structName, line)
		if err != nil {
			return nil, err
		}

		if field != nil {
			schema.Properties[field.jsonName] = field.schema
			if !field.optional {
				schema.Required = append(schema.Required, field.jsonName)
			}
		}
	}

	// Don't include empty required array
	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	return schema, nil
}

// fieldInfo holds parsed field information.
type fieldInfo struct {
	jsonName string
	schema   *JSONSchema
	optional bool
}

// Field parsing regex
var fieldRegex = regexp.MustCompile(`^\s*(\w+)\s+(\S+)(?:\s+` + "`" + `([^` + "`" + `]+)` + "`" + `)?\s*$`)

// parseField parses a single struct field.
func (p *GoStructParser) parseField(structName, line string) (*fieldInfo, error) {
	match := fieldRegex.FindStringSubmatch(line)
	if match == nil {
		// Skip unparseable lines (might be comments or malformed)
		return nil, nil
	}

	fieldName := match[1]
	fieldType := match[2]
	tags := match[3]

	// Parse JSON tag
	jsonName, omitempty := parseJSONTag(tags)
	if jsonName == "" {
		// Use field name with lowercase first letter if no json tag
		jsonName = strings.ToLower(fieldName[:1]) + fieldName[1:]
	}
	if jsonName == "-" {
		// Field should be skipped
		return nil, nil
	}

	// Parse type - may return error for forbidden types or add warnings
	schema, isPointer, err := p.parseType(fieldType, structName, fieldName)
	if err != nil {
		return nil, err
	}

	// Field is optional if it has omitempty or is a pointer
	optional := omitempty || isPointer

	return &fieldInfo{
		jsonName: jsonName,
		schema:   schema,
		optional: optional,
	}, nil
}

// parseJSONTag extracts the JSON field name and omitempty flag from struct tags.
func parseJSONTag(tags string) (name string, omitempty bool) {
	if tags == "" {
		return "", false
	}

	// Find json tag
	jsonTagRegex := regexp.MustCompile(`json:"([^"]*)"`)
	match := jsonTagRegex.FindStringSubmatch(tags)
	if match == nil {
		return "", false
	}

	parts := strings.Split(match[1], ",")
	name = parts[0]

	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitempty = true
		}
	}

	return name, omitempty
}

// Forbidden types that cannot be validated as JSON Schema
var forbiddenTypes = map[string]bool{
	"any":         true,
	"interface{}": true,
}

// Warning types that allow arbitrary data but can still be used
var warningTypes = map[string]bool{
	"json.RawMessage": true,
	"[]byte":          true,
}

// parseType parses a Go type and returns a JSON Schema.
// Returns the schema, whether the type is a pointer, and any error.
// Returns an error for forbidden types (any, interface{}).
// Adds warnings for types that allow arbitrary data (json.RawMessage, []byte).
// Pointer types generate nullable schemas (anyOf with null).
func (p *GoStructParser) parseType(typeStr, structName, fieldName string) (*JSONSchema, bool, error) {
	typeStr = strings.TrimSpace(typeStr)
	isPointer := false

	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		isPointer = true
		typeStr = strings.TrimPrefix(typeStr, "*")
	}

	// Check for forbidden types first (before processing slices/maps)
	if forbiddenTypes[typeStr] {
		return nil, false, fmt.Errorf("field %s.%s uses forbidden type %q: untyped values cannot be validated against a schema", structName, fieldName, typeStr)
	}

	// Check for warning types
	if warningTypes[typeStr] {
		p.warnings = append(p.warnings, fmt.Sprintf("field %s.%s uses type %q which allows arbitrary data and cannot be fully validated", structName, fieldName, typeStr))
		return &JSONSchema{}, isPointer, nil // Empty schema matches anything
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		elemType := strings.TrimPrefix(typeStr, "[]")

		// Special case: []byte is a warning type
		if elemType == "byte" {
			p.warnings = append(p.warnings, fmt.Sprintf("field %s.%s uses type \"[]byte\" which allows arbitrary data and cannot be fully validated", structName, fieldName))
			return &JSONSchema{}, isPointer, nil
		}

		elemSchema, _, err := p.parseType(elemType, structName, fieldName)
		if err != nil {
			return nil, false, err
		}
		schema := &JSONSchema{
			Type:  "array",
			Items: elemSchema,
		}
		return maybeNullable(schema, isPointer), isPointer, nil
	}

	// Handle map types
	if strings.HasPrefix(typeStr, "map[string]") {
		// Maps become objects with additionalProperties
		valueType := strings.TrimPrefix(typeStr, "map[string]")
		valueSchema, _, err := p.parseType(valueType, structName, fieldName)
		if err != nil {
			return nil, false, err
		}
		additionalProps := true
		schema := &JSONSchema{
			Type:                 "object",
			AdditionalProperties: &additionalProps,
			Items:                valueSchema, // Using Items to store additionalProperties schema
		}
		return maybeNullable(schema, isPointer), isPointer, nil
	}

	// Handle primitive types
	var schema *JSONSchema
	switch typeStr {
	case "string":
		schema = &JSONSchema{Type: "string"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		schema = &JSONSchema{Type: "integer"}
	case "float32", "float64":
		schema = &JSONSchema{Type: "number"}
	case "bool":
		schema = &JSONSchema{Type: "boolean"}
	default:
		// Check if it's a known struct type
		if _, known := p.structs[typeStr]; known {
			schema = &JSONSchema{Ref: "#/$defs/" + typeStr}
		} else {
			// Unknown type - treat as any (but add a warning)
			p.warnings = append(p.warnings, fmt.Sprintf("field %s.%s uses unknown type %q which will match any value", structName, fieldName, typeStr))
			return &JSONSchema{}, isPointer, nil
		}
	}

	return maybeNullable(schema, isPointer), isPointer, nil
}

// maybeNullable wraps a schema in anyOf with null if the field is a pointer (nullable).
func maybeNullable(schema *JSONSchema, isPointer bool) *JSONSchema {
	if !isPointer {
		return schema
	}
	// Create a nullable schema: anyOf with the original type and null
	return &JSONSchema{
		AnyOf: []*JSONSchema{
			schema,
			{Type: "null"},
		},
	}
}
