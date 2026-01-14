package types

// SchemaFormat represents the input format for schema definitions.
type SchemaFormat string

// Schema format constants.
const (
	FormatGoStruct   SchemaFormat = "go_struct"
	FormatZod        SchemaFormat = "zod"
	FormatJSONSchema SchemaFormat = "json_schema"
)

// ValidationResult contains the result of validating a single value.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}
