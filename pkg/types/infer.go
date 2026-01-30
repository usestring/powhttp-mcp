package types

import "github.com/usestring/powhttp-mcp/pkg/shape"

// InferSchemaOutput is the aggregate output type for the powhttp_infer_schema tool.
// It spans types from both pkg/jsonschema and pkg/shape.
type InferSchemaOutput struct {
	// Shape analysis result (contains content_category and format-specific fields)
	Shape *shape.Result `json:"shape"`

	// Summary of the inference process
	Summary InferSchemaSummary `json:"summary"`

	// Hint for the next step
	Hint string `json:"hint,omitempty"`
}

// InferSchemaSummary describes the inference process.
type InferSchemaSummary struct {
	EntriesRequested int    `json:"entries_requested"`
	EntriesProcessed int    `json:"entries_processed"`
	EntriesSkipped   int    `json:"entries_skipped"`
	ContentCategory  string `json:"content_category"`
}
