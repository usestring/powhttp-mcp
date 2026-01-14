// Package jsonschema provides JSON Schema inference from arbitrary JSON data.
// It generates schemas following JSON Schema Draft 2020-12.
package jsonschema

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/invopop/jsonschema"
)

// InferredSchema contains a JSON Schema inferred from sample data along with metadata.
type InferredSchema struct {
	Schema      *jsonschema.Schema `json:"schema"`       // JSON Schema (Draft 2020-12)
	SampleCount int                `json:"sample_count"` // Number of samples used
	AllMatch    bool               `json:"all_match"`    // True if all samples had identical schema
}

// InferOptions controls schema inference behavior.
type InferOptions struct {
	// StrictRequired marks properties as required only if present in ALL samples.
	// When true with multiple samples: only fields in ALL samples are required.
	// When true with single sample: ALL present fields are required.
	// When false: no fields are marked as required.
	// Default: true
	StrictRequired bool
	// AdditionalProperties sets additionalProperties in object schemas.
	// Default: nil (not set)
	AdditionalProperties *bool
	// MarkNullableAsOptional treats fields that can be null as optional (not required).
	// Default: true
	MarkNullableAsOptional bool
}

// DefaultInferOptions returns the default inference options.
func DefaultInferOptions() *InferOptions {
	return &InferOptions{
		StrictRequired:         true,
		AdditionalProperties:   nil,
		MarkNullableAsOptional: true,
	}
}

// Infer generates a JSON Schema from one or more JSON byte samples.
// Returns a merged schema if multiple samples are provided.
func Infer(samples ...[]byte) (*InferredSchema, error) {
	return InferWithOptions(DefaultInferOptions(), samples...)
}

// InferWithOptions generates a JSON Schema with custom options.
func InferWithOptions(opts *InferOptions, samples ...[]byte) (*InferredSchema, error) {
	if len(samples) == 0 {
		return nil, nil
	}

	if opts == nil {
		opts = DefaultInferOptions()
	}

	// Parse all samples
	parsedSamples := make([]any, 0, len(samples))
	for _, data := range samples {
		var parsed any
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		parsedSamples = append(parsedSamples, parsed)
	}

	if len(parsedSamples) == 0 {
		return nil, nil
	}

	// Infer schemas from each sample
	schemas := make([]*jsonschema.Schema, 0, len(parsedSamples))
	for _, parsed := range parsedSamples {
		schemas = append(schemas, inferFromValue(parsed))
	}

	// Check if all schemas are identical
	allMatch := true
	if len(schemas) > 1 {
		first, _ := json.Marshal(schemas[0])
		for i := 1; i < len(schemas); i++ {
			other, _ := json.Marshal(schemas[i])
			if string(first) != string(other) {
				allMatch = false
				break
			}
		}
	}

	// Merge schemas
	merged := mergeSchemas(schemas)

	// If strict required, compute required fields for object schemas
	if opts.StrictRequired && merged.Type == "object" {
		computeRequiredFields(merged, parsedSamples, opts.MarkNullableAsOptional)
	}

	// Apply additionalProperties setting
	if opts.AdditionalProperties != nil {
		applyAdditionalProperties(merged, *opts.AdditionalProperties)
	}

	return &InferredSchema{
		Schema:      merged,
		SampleCount: len(schemas),
		AllMatch:    allMatch,
	}, nil
}

// InferFromValue generates a JSON Schema from an already-parsed JSON value.
func InferFromValue(v any) *jsonschema.Schema {
	return inferFromValue(v)
}

func inferFromValue(v any) *jsonschema.Schema {
	if v == nil {
		return &jsonschema.Schema{Type: "null"}
	}

	switch val := v.(type) {
	case bool:
		return &jsonschema.Schema{Type: "boolean"}

	case float64:
		// JSON unmarshals all numbers as float64
		// Check if it's actually an integer (no fractional part)
		if math.Trunc(val) == val && !math.IsInf(val, 0) && !math.IsNaN(val) {
			return &jsonschema.Schema{Type: "integer"}
		}
		return &jsonschema.Schema{Type: "number"}

	case float32:
		f64 := float64(val)
		if math.Trunc(f64) == f64 && !math.IsInf(f64, 0) && !math.IsNaN(f64) {
			return &jsonschema.Schema{Type: "integer"}
		}
		return &jsonschema.Schema{Type: "number"}

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return &jsonschema.Schema{Type: "integer"}

	case string:
		return &jsonschema.Schema{Type: "string"}

	case []any:
		return inferArraySchema(val)

	case map[string]any:
		return inferObjectSchema(val)

	default:
		// Unknown type, return empty schema (matches anything)
		return &jsonschema.Schema{}
	}
}

func inferArraySchema(arr []any) *jsonschema.Schema {
	schema := &jsonschema.Schema{Type: "array"}

	if len(arr) == 0 {
		return schema
	}

	itemSchemas := make([]*jsonschema.Schema, 0, len(arr))
	for _, item := range arr {
		itemSchemas = append(itemSchemas, inferFromValue(item))
	}

	schema.Items = mergeSchemas(itemSchemas)
	return schema
}

func inferObjectSchema(obj map[string]any) *jsonschema.Schema {
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: jsonschema.NewProperties(),
	}

	if len(obj) == 0 {
		return schema
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		schema.Properties.Set(k, inferFromValue(obj[k]))
	}

	return schema
}

func mergeSchemas(schemas []*jsonschema.Schema) *jsonschema.Schema {
	if len(schemas) == 0 {
		return &jsonschema.Schema{}
	}
	if len(schemas) == 1 {
		return schemas[0]
	}

	// Collect all types
	types := make(map[string]bool)
	var objectSchemas []*jsonschema.Schema
	var arraySchemas []*jsonschema.Schema

	for _, s := range schemas {
		if s.Type == "" {
			continue
		}
		types[s.Type] = true

		if s.Type == "object" {
			objectSchemas = append(objectSchemas, s)
		}
		if s.Type == "array" {
			arraySchemas = append(arraySchemas, s)
		}
	}

	// All same type - merge appropriately
	if len(types) == 1 {
		for t := range types {
			switch t {
			case "object":
				return mergeObjectSchemas(objectSchemas)
			case "array":
				return mergeArraySchemas(arraySchemas)
			default:
				return schemas[0]
			}
		}
	}

	// Multiple types - check if all primitive
	typeList := make([]string, 0, len(types))
	for t := range types {
		typeList = append(typeList, t)
	}
	sort.Strings(typeList)

	allPrimitive := true
	for _, t := range typeList {
		if t == "object" || t == "array" {
			allPrimitive = false
			break
		}
	}

	// For primitive type unions, we could use type array but invopop/jsonschema
	// doesn't support that directly. Use anyOf instead for consistency.
	if allPrimitive && len(typeList) > 1 {
		anyOf := make([]*jsonschema.Schema, 0, len(typeList))
		for _, t := range typeList {
			anyOf = append(anyOf, &jsonschema.Schema{Type: t})
		}
		return &jsonschema.Schema{AnyOf: anyOf}
	}

	// Complex case - use anyOf
	anyOf := make([]*jsonschema.Schema, 0)
	if len(objectSchemas) > 0 {
		anyOf = append(anyOf, mergeObjectSchemas(objectSchemas))
	}
	if len(arraySchemas) > 0 {
		anyOf = append(anyOf, mergeArraySchemas(arraySchemas))
	}
	for _, t := range typeList {
		if t != "object" && t != "array" {
			anyOf = append(anyOf, &jsonschema.Schema{Type: t})
		}
	}

	if len(anyOf) == 1 {
		return anyOf[0]
	}
	return &jsonschema.Schema{AnyOf: anyOf}
}

func mergeObjectSchemas(schemas []*jsonschema.Schema) *jsonschema.Schema {
	if len(schemas) == 0 {
		return &jsonschema.Schema{Type: "object"}
	}
	if len(schemas) == 1 {
		return schemas[0]
	}

	// Merge properties from all schemas
	allProperties := make(map[string][]*jsonschema.Schema)
	for _, s := range schemas {
		if s.Properties == nil {
			continue
		}
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			allProperties[pair.Key] = append(allProperties[pair.Key], pair.Value)
		}
	}

	merged := &jsonschema.Schema{
		Type:       "object",
		Properties: jsonschema.NewProperties(),
	}

	keys := make([]string, 0, len(allProperties))
	for k := range allProperties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		merged.Properties.Set(k, mergeSchemas(allProperties[k]))
	}

	return merged
}

func mergeArraySchemas(schemas []*jsonschema.Schema) *jsonschema.Schema {
	if len(schemas) == 0 {
		return &jsonschema.Schema{Type: "array"}
	}
	if len(schemas) == 1 {
		return schemas[0]
	}

	itemSchemas := make([]*jsonschema.Schema, 0, len(schemas))
	for _, s := range schemas {
		if s.Items != nil {
			itemSchemas = append(itemSchemas, s.Items)
		}
	}

	return &jsonschema.Schema{
		Type:  "array",
		Items: mergeSchemas(itemSchemas),
	}
}

// computeRequiredFields determines which properties are present in ALL samples
// and marks them as required in the schema.
// If markNullableAsOptional is true, fields that contain null values are not marked as required.
func computeRequiredFields(schema *jsonschema.Schema, samples []any, markNullableAsOptional bool) {
	if schema.Type != "object" || schema.Properties == nil {
		return
	}

	// Count occurrences of each property across all samples
	// Also track if the property ever has a null value
	propCounts := make(map[string]int)
	propNullable := make(map[string]bool)
	for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		propCounts[pair.Key] = 0
		propNullable[pair.Key] = false
	}

	for _, sample := range samples {
		obj, ok := sample.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range obj {
			if _, exists := propCounts[key]; exists {
				propCounts[key]++
				// Check if value is null
				if value == nil {
					propNullable[key] = true
				}
			}
		}
	}

	// Properties present in all samples are required
	// Unless they're nullable and markNullableAsOptional is true
	totalSamples := len(samples)
	required := make([]string, 0)
	for key, count := range propCounts {
		if count == totalSamples {
			// If field can be null and we're marking nullable as optional, skip it
			if markNullableAsOptional && propNullable[key] {
				continue
			}
			required = append(required, key)
		}
	}
	sort.Strings(required)

	if len(required) > 0 {
		schema.Required = required
	}

	// Recursively handle nested objects
	for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		propSchema := pair.Value
		if propSchema.Type == "object" {
			// Collect nested samples for this property
			nestedSamples := make([]any, 0)
			for _, sample := range samples {
				obj, ok := sample.(map[string]any)
				if !ok {
					continue
				}
				if nested, exists := obj[pair.Key]; exists && nested != nil {
					nestedSamples = append(nestedSamples, nested)
				}
			}
			if len(nestedSamples) > 0 {
				computeRequiredFields(propSchema, nestedSamples, markNullableAsOptional)
			}
		} else if propSchema.Type == "array" && propSchema.Items != nil && propSchema.Items.Type == "object" {
			// For arrays of objects, collect all items from all samples
			nestedSamples := make([]any, 0)
			for _, sample := range samples {
				obj, ok := sample.(map[string]any)
				if !ok {
					continue
				}
				if arr, exists := obj[pair.Key]; exists {
					if items, ok := arr.([]any); ok {
						for _, item := range items {
							if item != nil {
								nestedSamples = append(nestedSamples, item)
							}
						}
					}
				}
			}
			if len(nestedSamples) > 0 {
				computeRequiredFields(propSchema.Items, nestedSamples, markNullableAsOptional)
			}
		}
	}
}

// applyAdditionalProperties recursively sets additionalProperties on all object schemas.
func applyAdditionalProperties(schema *jsonschema.Schema, allowed bool) {
	if schema == nil {
		return
	}

	if schema.Type == "object" {
		if allowed {
			schema.AdditionalProperties = jsonschema.TrueSchema
		} else {
			schema.AdditionalProperties = jsonschema.FalseSchema
		}

		// Recurse into properties
		if schema.Properties != nil {
			for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
				applyAdditionalProperties(pair.Value, allowed)
			}
		}
	}

	// Recurse into array items
	if schema.Type == "array" && schema.Items != nil {
		applyAdditionalProperties(schema.Items, allowed)
	}

	// Recurse into anyOf schemas
	for _, s := range schema.AnyOf {
		applyAdditionalProperties(s, allowed)
	}
}
