package tools

import (
	"context"
	"encoding/base64"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/schema"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ValidateSchemaInput is the input for powhttp_validate_schema.
type ValidateSchemaInput struct {
	SessionID    string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	ClusterID    string   `json:"cluster_id,omitempty" jsonschema:"Validate all entries in this cluster"`
	EntryIDs     []string `json:"entry_ids,omitempty" jsonschema:"Validate these specific entries"`
	Schema       string   `json:"schema" jsonschema:"required,Schema definition (Go struct, Zod, or JSON Schema)"`
	SchemaFormat string   `json:"schema_format" jsonschema:"required,Format: go_struct, zod, or json_schema"`
	Target       string   `json:"target,omitempty" jsonschema:"Which body to validate: request, response, or both (default: response)"`
	MaxEntries   int      `json:"max_entries,omitempty" jsonschema:"Max entries to validate (default: 20)"`
}

// ValidateSchemaOutput is the output for powhttp_validate_schema.
type ValidateSchemaOutput struct {
	Summary      ValidationSummary `json:"summary"`
	Results      []EntryValidation `json:"results,omitzero"`
	CommonErrors []CommonError     `json:"common_errors,omitempty"`
	ParsedSchema any               `json:"parsed_schema,omitempty"`
}

// CommonError represents a frequently occurring validation error.
type CommonError struct {
	Error     string `json:"error"`
	Frequency int    `json:"frequency"`
}

// ValidationSummary summarizes the validation results.
type ValidationSummary struct {
	TotalEntries  int  `json:"total_entries"`
	MatchingCount int  `json:"matching_count"`
	FailedCount   int  `json:"failed_count"`
	SkippedCount  int  `json:"skipped_count"`
	AllMatch      bool `json:"all_match"`
}

// EntryValidation contains the validation result for a single entry.
type EntryValidation struct {
	EntryID    string   `json:"entry_id"`
	Target     string   `json:"target"`
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	Skipped    bool     `json:"skipped,omitempty"`
	SkipReason string   `json:"skip_reason,omitempty"`
}

// ToolValidateSchema validates HTTP entry bodies against a schema.
func ToolValidateSchema(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ValidateSchemaInput) (*sdkmcp.CallToolResult, ValidateSchemaOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ValidateSchemaInput) (*sdkmcp.CallToolResult, ValidateSchemaOutput, error) {
		if input.Schema == "" {
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("schema is required")
		}
		if input.SchemaFormat == "" {
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("schema_format is required")
		}

		if input.ClusterID == "" && len(input.EntryIDs) == 0 {
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("either cluster_id or entry_ids is required")
		}

		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, ValidateSchemaOutput{}, err
		}

		target := input.Target
		if target == "" {
			target = "response"
		}
		if target != "request" && target != "response" && target != "both" {
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("target must be 'request', 'response', or 'both'")
		}

		var schemaFormat types.SchemaFormat
		switch input.SchemaFormat {
		case "go_struct":
			schemaFormat = types.FormatGoStruct
		case "zod":
			schemaFormat = types.FormatZod
		case "json_schema":
			schemaFormat = types.FormatJSONSchema
		default:
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("schema_format must be 'go_struct', 'zod', or 'json_schema'")
		}

		validator, err := schema.NewValidator(input.Schema, schemaFormat)
		if err != nil {
			return nil, ValidateSchemaOutput{}, ErrInvalidInput("invalid schema: " + err.Error())
		}

		var entryIDs []string
		if input.ClusterID != "" {
			stored, ok := d.ClusterStore.GetCluster(input.ClusterID)
			if !ok {
				return nil, ValidateSchemaOutput{}, ErrNotFound("cluster", input.ClusterID)
			}
			entryIDs = stored.EntryIDs
		} else {
			entryIDs = input.EntryIDs
		}

		maxEntries := input.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 20
		}
		if len(entryIDs) > maxEntries {
			entryIDs = entryIDs[:maxEntries]
		}

		results := make([]EntryValidation, 0, len(entryIDs)*2)
		summary := ValidationSummary{
			TotalEntries: len(entryIDs),
		}

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				results = append(results, EntryValidation{
					EntryID:    entryID,
					Target:     target,
					Skipped:    true,
					SkipReason: "failed to fetch entry: " + err.Error(),
				})
				summary.SkippedCount++
				continue
			}

			if target == "request" || target == "both" {
				result := validateEntryBody(validator, entry, "request")
				result.EntryID = entryID
				results = append(results, result)

				if result.Skipped {
					summary.SkippedCount++
				} else if result.Valid {
					summary.MatchingCount++
				} else {
					summary.FailedCount++
				}
			}

			if target == "response" || target == "both" {
				result := validateEntryBody(validator, entry, "response")
				result.EntryID = entryID
				results = append(results, result)

				if result.Skipped {
					summary.SkippedCount++
				} else if result.Valid {
					summary.MatchingCount++
				} else {
					summary.FailedCount++
				}
			}
		}

		validatedCount := summary.MatchingCount + summary.FailedCount
		summary.AllMatch = validatedCount > 0 && summary.FailedCount == 0

		// Aggregate common errors
		var commonErrors []CommonError
		if summary.FailedCount > 0 {
			errorCounts := make(map[string]int)
			for _, r := range results {
				for _, e := range r.Errors {
					errorCounts[e]++
				}
			}
			commonErrors = make([]CommonError, 0, len(errorCounts))
			for e, count := range errorCounts {
				commonErrors = append(commonErrors, CommonError{Error: e, Frequency: count})
			}
			sort.Slice(commonErrors, func(i, j int) bool {
				return commonErrors[i].Frequency > commonErrors[j].Frequency
			})
		}

		output := ValidateSchemaOutput{
			Summary:      summary,
			Results:      results,
			CommonErrors: commonErrors,
		}

		return nil, output, nil
	}
}

func validateEntryBody(validator *schema.Validator, entry *client.SessionEntry, target string) EntryValidation {
	result := EntryValidation{
		Target: target,
	}

	var body *string
	var contentType string

	if target == "request" {
		body = entry.Request.Body
		contentType = entry.Request.Headers.Get("content-type")
	} else {
		if entry.Response == nil {
			result.Skipped = true
			result.SkipReason = "no response"
			return result
		}
		body = entry.Response.Body
		contentType = entry.Response.Headers.Get("content-type")
	}

	if body == nil || *body == "" {
		result.Skipped = true
		result.SkipReason = "no body"
		return result
	}

	if !contenttype.IsJSON(contentType) {
		result.Skipped = true
		result.SkipReason = "not JSON content-type: " + contentType
		return result
	}

	bodyBytes, err := base64.StdEncoding.DecodeString(*body)
	if err != nil {
		result.Skipped = true
		result.SkipReason = "failed to decode body: " + err.Error()
		return result
	}

	validationResult := validator.Validate(bodyBytes)
	result.Valid = validationResult.Valid
	result.Errors = validationResult.Errors

	return result
}
