package jsonschema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"
)

// FieldStat contains per-field statistics computed across multiple JSON samples.
type FieldStat struct {
	Path          string   `json:"path"`                     // JSON path (e.g., "user.name", "items[].id")
	Type          string   `json:"type"`                     // JSON Schema type (string, number, object, array, etc.)
	Frequency     float64  `json:"frequency"`                // Fraction of samples containing this field (0.0-1.0)
	Required      bool     `json:"required"`                 // Present in all samples and never null
	Nullable      bool     `json:"nullable"`                 // At least one sample has null for this field
	DistinctCount int      `json:"distinct_count"`           // Number of distinct non-null values observed
	Examples      []any    `json:"examples"`                 // Up to 3 example values
	Format        string   `json:"format,omitempty"`         // Detected format: uuid, iso8601, url, email, enum
	EnumValues    []string `json:"enum_values,omitempty"`    // All distinct values when format is "enum"
}

const (
	defaultMaxDepth       = 5
	maxExamples           = 3
	minSamplesForFormat   = 5
	maxEnumDistinctValues = 10
)

var (
	uuidRegex    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	iso8601Regex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2})?`)
	urlRegex     = regexp.MustCompile(`^https?://`)
	emailRegex   = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)
)

// ComputeFieldStats walks the merged schema and computes per-field statistics
// by cross-referencing raw samples. Returns a flat table of field stats.
func ComputeFieldStats(schema *jsonschema.Schema, samples [][]byte) []FieldStat {
	if schema == nil || len(samples) == 0 {
		return nil
	}

	// Parse all samples
	parsed := make([]any, 0, len(samples))
	for _, s := range samples {
		var v any
		if err := json.Unmarshal(s, &v); err != nil {
			continue
		}
		parsed = append(parsed, v)
	}

	if len(parsed) == 0 {
		return nil
	}

	var stats []FieldStat
	walkSchema(schema, "", parsed, 0, defaultMaxDepth, &stats)
	return stats
}

// walkSchema recursively walks the schema and collects field stats.
func walkSchema(schema *jsonschema.Schema, path string, samples []any, depth, maxDepth int, stats *[]FieldStat) {
	if schema == nil || depth > maxDepth {
		if depth > maxDepth && path != "" {
			*stats = append(*stats, FieldStat{
				Path: path + " (truncated at depth limit)",
				Type: "...",
			})
		}
		return
	}

	if schema.Type == "object" && schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			propName := pair.Key
			propSchema := pair.Value

			fieldPath := propName
			if path != "" {
				fieldPath = path + "." + propName
			}

			stat := computeSingleFieldStat(fieldPath, propSchema, propName, samples)
			*stats = append(*stats, stat)

			// Recurse into nested objects
			if propSchema.Type == "object" && propSchema.Properties != nil {
				nestedSamples := collectNestedSamples(propName, samples)
				walkSchema(propSchema, fieldPath, nestedSamples, depth+1, maxDepth, stats)
			}

			// Recurse into array items
			if propSchema.Type == "array" && propSchema.Items != nil {
				if propSchema.Items.Type == "object" && propSchema.Items.Properties != nil {
					arrayItemSamples := collectArrayItemSamples(propName, samples)
					walkSchema(propSchema.Items, fieldPath+"[]", arrayItemSamples, depth+1, maxDepth, stats)
				}
			}
		}
	}
}

// computeSingleFieldStat computes statistics for a single field across all samples.
func computeSingleFieldStat(path string, schema *jsonschema.Schema, fieldName string, samples []any) FieldStat {
	totalSamples := len(samples)
	stat := FieldStat{
		Path: path,
		Type: resolveType(schema),
	}

	presentCount := 0
	nullCount := 0
	distinctValues := make(map[string]bool)
	var examples []any
	var stringValues []string

	for _, sample := range samples {
		obj, ok := sample.(map[string]any)
		if !ok {
			continue
		}

		val, exists := obj[fieldName]
		if !exists {
			continue
		}

		presentCount++

		if val == nil {
			nullCount++
			continue
		}

		// Track distinct values and examples.
		// Skip collecting examples for objects and arrays — child field stats
		// describe their structure, so embedding full nested values is redundant.
		key := fmt.Sprintf("%v", val)
		if !distinctValues[key] {
			distinctValues[key] = true
			switch val.(type) {
			case map[string]any, []any:
				// count distinct but skip example collection
			default:
				if len(examples) < maxExamples {
					examples = append(examples, val)
				}
			}
		}

		// Collect string values for format detection
		if str, ok := val.(string); ok {
			stringValues = append(stringValues, str)
		}
	}

	if totalSamples > 0 {
		stat.Frequency = float64(presentCount) / float64(totalSamples)
	}
	stat.Required = presentCount == totalSamples && nullCount == 0
	stat.Nullable = nullCount > 0
	stat.DistinctCount = len(distinctValues)
	stat.Examples = examples

	// Format detection for string fields
	if stat.Type == "string" && len(stringValues) >= minSamplesForFormat {
		stat.Format, stat.EnumValues = detectFormat(stringValues)
	}

	return stat
}

// detectFormat detects common value formats for string fields.
func detectFormat(values []string) (string, []string) {
	if len(values) == 0 {
		return "", nil
	}

	// Check UUID
	allMatch := true
	for _, v := range values {
		if !uuidRegex.MatchString(v) {
			allMatch = false
			break
		}
	}
	if allMatch {
		return "uuid", nil
	}

	// Check ISO 8601
	allMatch = true
	for _, v := range values {
		if !iso8601Regex.MatchString(v) {
			allMatch = false
			break
		}
	}
	if allMatch {
		return "iso8601", nil
	}

	// Check URL
	allMatch = true
	for _, v := range values {
		if !urlRegex.MatchString(v) {
			allMatch = false
			break
		}
	}
	if allMatch {
		return "url", nil
	}

	// Check Email
	allMatch = true
	for _, v := range values {
		if !emailRegex.MatchString(v) {
			allMatch = false
			break
		}
	}
	if allMatch {
		return "email", nil
	}

	// Check Enum: <=10 distinct values
	distinct := make(map[string]bool)
	for _, v := range values {
		distinct[v] = true
	}
	if len(distinct) <= maxEnumDistinctValues {
		enumValues := make([]string, 0, len(distinct))
		for v := range distinct {
			enumValues = append(enumValues, v)
		}
		sort.Strings(enumValues)
		return "enum", enumValues
	}

	return "", nil
}

// collectNestedSamples extracts the value of a field from each sample object.
func collectNestedSamples(fieldName string, samples []any) []any {
	var nested []any
	for _, sample := range samples {
		obj, ok := sample.(map[string]any)
		if !ok {
			continue
		}
		if val, exists := obj[fieldName]; exists && val != nil {
			nested = append(nested, val)
		}
	}
	return nested
}

// collectArrayItemSamples extracts all array items from a field across samples.
func collectArrayItemSamples(fieldName string, samples []any) []any {
	var items []any
	for _, sample := range samples {
		obj, ok := sample.(map[string]any)
		if !ok {
			continue
		}
		if val, exists := obj[fieldName]; exists {
			if arr, ok := val.([]any); ok {
				for _, item := range arr {
					if item != nil {
						items = append(items, item)
					}
				}
			}
		}
	}
	return items
}

// resolveType returns the type string for a schema, handling anyOf unions.
func resolveType(schema *jsonschema.Schema) string {
	if schema.Type != "" {
		return schema.Type
	}
	if len(schema.AnyOf) > 0 {
		types := make([]string, 0, len(schema.AnyOf))
		for _, s := range schema.AnyOf {
			if s.Type != "" {
				types = append(types, s.Type)
			}
		}
		return strings.Join(types, "|")
	}
	return "unknown"
}
