package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseZodSchema parses a Zod schema definition and returns a JSON Schema.
//
// Supported Zod types:
//   - z.string()
//   - z.number()
//   - z.boolean()
//   - z.null()
//   - z.array(schema)
//   - z.object({ ... })
//   - z.record(schema) - becomes object with additionalProperties
//
// Supported modifiers:
//   - .optional()
//   - .nullable()
//   - .default(...)
//
// Returns an error if the schema contains forbidden types (z.any(), z.unknown()).
//
// Example input:
//
//	z.object({
//	  status: z.string(),
//	  data: z.array(z.object({
//	    id: z.number(),
//	    name: z.string().optional()
//	  }))
//	})
func ParseZodSchema(input string) (*JSONSchema, error) {
	parser := &zodParser{
		input: normalizeZodInput(input),
		pos:   0,
	}

	schema, err := parser.parse()
	if err != nil {
		return nil, fmt.Errorf("parsing zod schema: %w", err)
	}

	return schema, nil
}

// normalizeZodInput cleans up the input for easier parsing.
func normalizeZodInput(input string) string {
	// Remove escaped newlines
	input = strings.ReplaceAll(input, "\\n", "\n")
	// Normalize whitespace
	input = strings.TrimSpace(input)
	return input
}

type zodParser struct {
	input string
	pos   int
}

func (p *zodParser) parse() (*JSONSchema, error) {
	p.skipWhitespace()

	// Look for z.something()
	if !p.match("z.") {
		return nil, fmt.Errorf("expected 'z.' at position %d", p.pos)
	}

	return p.parseZodType()
}

func (p *zodParser) parseZodType() (*JSONSchema, error) {
	// Read the type name
	typeName := p.readIdentifier()
	if typeName == "" {
		return nil, fmt.Errorf("expected type name at position %d", p.pos)
	}

	var schema *JSONSchema
	var err error

	switch typeName {
	case "string":
		if !p.match("()") {
			return nil, fmt.Errorf("expected '()' after z.string at position %d", p.pos)
		}
		schema = &JSONSchema{Type: "string"}

	case "number":
		if !p.match("()") {
			return nil, fmt.Errorf("expected '()' after z.number at position %d", p.pos)
		}
		schema = &JSONSchema{Type: "number"}

	case "boolean":
		if !p.match("()") {
			return nil, fmt.Errorf("expected '()' after z.boolean at position %d", p.pos)
		}
		schema = &JSONSchema{Type: "boolean"}

	case "null":
		if !p.match("()") {
			return nil, fmt.Errorf("expected '()' after z.null at position %d", p.pos)
		}
		schema = &JSONSchema{Type: "null"}

	case "any", "unknown":
		return nil, fmt.Errorf("z.%s() is forbidden: untyped values cannot be validated against a schema", typeName)

	case "array":
		schema, err = p.parseZodArray()
		if err != nil {
			return nil, err
		}

	case "object":
		schema, err = p.parseZodObject()
		if err != nil {
			return nil, err
		}

	case "record":
		schema, err = p.parseZodRecord()
		if err != nil {
			return nil, err
		}

	case "enum":
		schema, err = p.parseZodEnum()
		if err != nil {
			return nil, err
		}

	case "literal":
		schema, err = p.parseZodLiteral()
		if err != nil {
			return nil, err
		}

	case "union":
		schema, err = p.parseZodUnion()
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unknown zod type: z.%s at position %d", typeName, p.pos)
	}

	// Parse modifiers
	return p.parseModifiers(schema)
}

func (p *zodParser) parseZodArray() (*JSONSchema, error) {
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.array at position %d", p.pos)
	}

	p.skipWhitespace()

	// Parse the item schema
	itemSchema, err := p.parse()
	if err != nil {
		return nil, fmt.Errorf("parsing array items: %w", err)
	}

	p.skipWhitespace()
	if !p.match(")") {
		return nil, fmt.Errorf("expected ')' after array items at position %d", p.pos)
	}

	return &JSONSchema{
		Type:  "array",
		Items: itemSchema,
	}, nil
}

func (p *zodParser) parseZodObject() (*JSONSchema, error) {
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.object at position %d", p.pos)
	}
	p.skipWhitespace()
	if !p.match("{") {
		return nil, fmt.Errorf("expected '{' after z.object( at position %d", p.pos)
	}

	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
		Required:   make([]string, 0),
	}

	// Parse properties
	for {
		p.skipWhitespace()

		// Check for end of object
		if p.peek() == '}' {
			p.pos++
			break
		}

		// Skip trailing commas
		if p.peek() == ',' {
			p.pos++
			continue
		}

		// Parse property name
		propName := p.readPropertyName()
		if propName == "" {
			return nil, fmt.Errorf("expected property name at position %d", p.pos)
		}

		p.skipWhitespace()
		if !p.match(":") {
			return nil, fmt.Errorf("expected ':' after property name at position %d", p.pos)
		}
		p.skipWhitespace()

		// Parse property schema
		propSchema, err := p.parse()
		if err != nil {
			return nil, fmt.Errorf("parsing property %s: %w", propName, err)
		}

		schema.Properties[propName] = propSchema

		// Properties without .optional() are required
		// We'll handle this in parseModifiers by marking schema
		schema.Required = append(schema.Required, propName)

		p.skipWhitespace()

		// Skip comma if present
		if p.peek() == ',' {
			p.pos++
		}
	}

	p.skipWhitespace()
	if !p.match(")") {
		return nil, fmt.Errorf("expected ')' after object definition at position %d", p.pos)
	}

	// Don't include empty required array
	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	return schema, nil
}

func (p *zodParser) parseZodRecord() (*JSONSchema, error) {
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.record at position %d", p.pos)
	}

	p.skipWhitespace()

	// Parse the value schema
	valueSchema, err := p.parse()
	if err != nil {
		return nil, fmt.Errorf("parsing record value: %w", err)
	}

	p.skipWhitespace()
	if !p.match(")") {
		return nil, fmt.Errorf("expected ')' after record value at position %d", p.pos)
	}

	additionalProps := true
	return &JSONSchema{
		Type:                 "object",
		AdditionalProperties: &additionalProps,
		Items:                valueSchema, // Store additionalProperties schema
	}, nil
}

func (p *zodParser) parseZodEnum() (*JSONSchema, error) {
	// z.enum(["a", "b", "c"])
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.enum at position %d", p.pos)
	}
	p.skipWhitespace()
	if !p.match("[") {
		return nil, fmt.Errorf("expected '[' after z.enum( at position %d", p.pos)
	}

	// For simplicity, just return a string type
	// A full implementation would parse the enum values
	p.skipUntil(']')
	p.match("]")
	p.skipWhitespace()
	p.match(")")

	return &JSONSchema{Type: "string"}, nil
}

func (p *zodParser) parseZodLiteral() (*JSONSchema, error) {
	// z.literal("value") or z.literal(123)
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.literal at position %d", p.pos)
	}
	p.skipWhitespace()

	// For simplicity, just detect the type and return that
	ch := p.peek()
	var schema *JSONSchema

	switch {
	case ch == '"' || ch == '\'':
		schema = &JSONSchema{Type: "string"}
	case ch >= '0' && ch <= '9':
		schema = &JSONSchema{Type: "number"}
	case p.matchWord("true") || p.matchWord("false"):
		schema = &JSONSchema{Type: "boolean"}
	case p.matchWord("null"):
		schema = &JSONSchema{Type: "null"}
	default:
		schema = &JSONSchema{}
	}

	p.skipUntil(')')
	p.match(")")

	return schema, nil
}

func (p *zodParser) parseZodUnion() (*JSONSchema, error) {
	// z.union([z.string(), z.number()])
	if !p.match("(") {
		return nil, fmt.Errorf("expected '(' after z.union at position %d", p.pos)
	}
	p.skipWhitespace()
	if !p.match("[") {
		return nil, fmt.Errorf("expected '[' after z.union( at position %d", p.pos)
	}

	anyOf := make([]*JSONSchema, 0)

	for {
		p.skipWhitespace()

		if p.peek() == ']' {
			p.pos++
			break
		}

		if p.peek() == ',' {
			p.pos++
			continue
		}

		schema, err := p.parse()
		if err != nil {
			return nil, fmt.Errorf("parsing union member: %w", err)
		}
		anyOf = append(anyOf, schema)

		p.skipWhitespace()
		if p.peek() == ',' {
			p.pos++
		}
	}

	p.skipWhitespace()
	if !p.match(")") {
		return nil, fmt.Errorf("expected ')' after union at position %d", p.pos)
	}

	if len(anyOf) == 1 {
		return anyOf[0], nil
	}

	return &JSONSchema{AnyOf: anyOf}, nil
}

func (p *zodParser) parseModifiers(schema *JSONSchema) (*JSONSchema, error) {
	for {
		p.skipWhitespace()
		if !p.match(".") {
			break
		}

		modifier := p.readIdentifier()
		switch modifier {
		case "optional":
			if !p.match("()") {
				return nil, fmt.Errorf("expected '()' after .optional at position %d", p.pos)
			}
			// Mark as optional by creating anyOf with null
			// Actually, for required tracking, we need a different approach
			// For now, we'll use a special marker in the schema
			// The caller should handle removing from required list
			schema = markOptional(schema)

		case "nullable":
			if !p.match("()") {
				return nil, fmt.Errorf("expected '()' after .nullable at position %d", p.pos)
			}
			// Create anyOf with null
			if schema.Type != "" {
				schema = &JSONSchema{
					AnyOf: []*JSONSchema{
						schema,
						{Type: "null"},
					},
				}
			}

		case "default":
			// Skip the default value
			if !p.match("(") {
				return nil, fmt.Errorf("expected '(' after .default at position %d", p.pos)
			}
			p.skipUntilBalanced('(', ')')
			// Default values make fields optional
			schema = markOptional(schema)

		case "describe", "min", "max", "length", "email", "url", "uuid", "regex",
			"int", "positive", "negative", "nonnegative", "nonpositive",
			"finite", "safe", "transform", "refine", "superRefine", "catch",
			"brand", "readonly", "pipe", "trim", "toLowerCase", "toUpperCase":
			// Skip these modifiers - they don't affect the schema type
			if p.peek() == '(' {
				p.match("(")
				p.skipUntilBalanced('(', ')')
			}

		default:
			// Unknown modifier, skip if it has parentheses
			if p.peek() == '(' {
				p.match("(")
				p.skipUntilBalanced('(', ')')
			}
		}
	}

	return schema, nil
}

// Helper functions

func (p *zodParser) skipWhitespace() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *zodParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *zodParser) match(s string) bool {
	if p.pos+len(s) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(s)] == s {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *zodParser) matchWord(s string) bool {
	if p.pos+len(s) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(s)] == s {
		// Check that next char is not alphanumeric
		if p.pos+len(s) < len(p.input) {
			next := p.input[p.pos+len(s)]
			if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_' {
				return false
			}
		}
		p.pos += len(s)
		return true
	}
	return false
}

func (p *zodParser) readIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos]
}

var propertyNameRegex = regexp.MustCompile(`^[\w]+`)
var quotedPropertyRegex = regexp.MustCompile(`^["']([^"']+)["']`)

func (p *zodParser) readPropertyName() string {
	p.skipWhitespace()

	// Check for quoted property name
	if p.peek() == '"' || p.peek() == '\'' {
		remaining := p.input[p.pos:]
		match := quotedPropertyRegex.FindStringSubmatch(remaining)
		if match != nil {
			p.pos += len(match[0])
			return match[1]
		}
	}

	// Unquoted property name
	remaining := p.input[p.pos:]
	match := propertyNameRegex.FindString(remaining)
	if match != "" {
		p.pos += len(match)
		return match
	}

	return ""
}

func (p *zodParser) skipUntil(ch byte) {
	for p.pos < len(p.input) && p.input[p.pos] != ch {
		p.pos++
	}
}

func (p *zodParser) skipUntilBalanced(open, close byte) {
	depth := 1
	for p.pos < len(p.input) && depth > 0 {
		ch := p.input[p.pos]
		if ch == open {
			depth++
		} else if ch == close {
			depth--
		}
		p.pos++
	}
}

// OptionalMarker is used to track if a schema was marked as optional.
// This is a workaround since JSONSchema doesn't have an optional field.
const optionalMarker = "__zod_optional__"

func markOptional(schema *JSONSchema) *JSONSchema {
	// Add a marker to track optionality
	// This will be handled when building the parent object's required array
	if schema.Ref == "" && schema.Ref != optionalMarker {
		schema.Ref = optionalMarker
	}
	return schema
}

// IsOptional checks if a schema was marked as optional during Zod parsing.
func IsOptional(schema *JSONSchema) bool {
	return schema != nil && schema.Ref == optionalMarker
}

// CleanOptionalMarkers removes optional markers from the schema.
func CleanOptionalMarkers(schema *JSONSchema) {
	if schema == nil {
		return
	}

	if schema.Ref == optionalMarker {
		schema.Ref = ""
	}

	for _, prop := range schema.Properties {
		CleanOptionalMarkers(prop)
	}

	if schema.Items != nil {
		CleanOptionalMarkers(schema.Items)
	}

	for _, s := range schema.AnyOf {
		CleanOptionalMarkers(s)
	}
}

// PostProcessZodSchema cleans up a Zod-parsed schema.
// It removes optional markers and updates required arrays.
func PostProcessZodSchema(schema *JSONSchema) *JSONSchema {
	if schema == nil {
		return nil
	}

	// Process object properties
	if schema.Type == "object" && schema.Properties != nil {
		newRequired := make([]string, 0)
		for name, prop := range schema.Properties {
			// Check if marked optional
			if !IsOptional(prop) {
				// Check if name was in original required list
				for _, r := range schema.Required {
					if r == name {
						newRequired = append(newRequired, name)
						break
					}
				}
			}
			// Recursively process
			schema.Properties[name] = PostProcessZodSchema(prop)
		}
		if len(newRequired) > 0 {
			schema.Required = newRequired
		} else {
			schema.Required = nil
		}
	}

	// Process array items
	if schema.Items != nil {
		schema.Items = PostProcessZodSchema(schema.Items)
	}

	// Process anyOf
	for i, s := range schema.AnyOf {
		schema.AnyOf[i] = PostProcessZodSchema(s)
	}

	// Process definitions
	for name, def := range schema.Definitions {
		schema.Definitions[name] = PostProcessZodSchema(def)
	}

	// Clean optional marker
	if schema.Ref == optionalMarker {
		schema.Ref = ""
	}

	return schema
}
