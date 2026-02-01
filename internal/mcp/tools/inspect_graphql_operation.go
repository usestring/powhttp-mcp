package tools

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/graphql"
	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

// ---------------------------------------------------------------------------
// Input / Output types
// ---------------------------------------------------------------------------

// InspectGraphQLOperationInput is the input for powhttp_inspect_graphql_operation.
type InspectGraphQLOperationInput struct {
	SessionID     string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs      []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to inspect. When both entry_ids and operation_name are provided, only the named operation within the given entries is inspected. Either entry_ids or operation_name is required."`
	OperationName string   `json:"operation_name,omitempty" jsonschema:"GraphQL operation name to find and inspect. Either entry_ids or operation_name is required."`
	Host          string   `json:"host,omitempty" jsonschema:"Filter search by host (used with operation_name; ignored when entry_ids is provided). Prefix with '*.' to include subdomains (e.g., '*.example.com'). Prefer '*.domain' to capture all related traffic."`
	Sections      []string `json:"sections,omitempty" jsonschema:"Which sections to include: query, variables, response_shape, errors. Default: all four."`
	MaxEntries    int      `json:"max_entries,omitempty" jsonschema:"Max entries to inspect (default: 20, max: 100)"`
}

// InspectGraphQLOperationOutput is the output for powhttp_inspect_graphql_operation.
type InspectGraphQLOperationOutput struct {
	OperationName  string                `json:"operation_name"`
	OperationType  string                `json:"operation_type,omitempty"`
	Query          string                `json:"query,omitempty"`
	FieldStats     []js.FieldStat        `json:"field_stats,omitempty"`
	ErrorGroups    []graphql.ErrorGroup  `json:"error_groups,omitzero"`
	ErrorSummary   *graphql.ErrorSummary `json:"error_summary,omitempty"`
	EntriesMatched int                   `json:"entries_matched"`
	EntryIDs       []string              `json:"entry_ids,omitempty"`
	Resources      map[string]string     `json:"resources,omitempty"`
	Hint           string                `json:"hint,omitempty"`
}

// inspectSections tracks which sections are requested.
type inspectSections struct {
	query         bool
	variables     bool
	responseShape bool
	errors        bool
}

// allSections returns an inspectSections with everything enabled.
func allSections() inspectSections {
	return inspectSections{
		query:         true,
		variables:     true,
		responseShape: true,
		errors:        true,
	}
}

// parseSections validates and parses the sections parameter.
func parseSections(input []string) (inspectSections, error) {
	if len(input) == 0 {
		return allSections(), nil
	}

	s := inspectSections{}
	for _, v := range input {
		switch v {
		case "query":
			s.query = true
		case "variables":
			s.variables = true
		case "response_shape":
			s.responseShape = true
		case "errors":
			s.errors = true
		default:
			return s, ErrInvalidInput(fmt.Sprintf("invalid section %q: valid values are query, variables, response_shape, errors", v))
		}
	}
	return s, nil
}

// needsInspect returns true if any inspect-related section is requested.
func (s inspectSections) needsInspect() bool {
	return s.query || s.variables || s.responseShape
}

// ---------------------------------------------------------------------------
// Tool: powhttp_inspect_graphql_operation
// ---------------------------------------------------------------------------

// ToolInspectGraphQLOperation inspects a single GraphQL operation: parses
// the query, infers schemas, computes field stats, and collects errors.
// Merges the functionality of the old graphql_inspect and graphql_errors tools.
func ToolInspectGraphQLOperation(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input InspectGraphQLOperationInput) (*sdkmcp.CallToolResult, InspectGraphQLOperationOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input InspectGraphQLOperationInput) (*sdkmcp.CallToolResult, InspectGraphQLOperationOutput, error) {
		if len(input.EntryIDs) == 0 && input.OperationName == "" {
			return nil, InspectGraphQLOperationOutput{}, ErrInvalidInput("either entry_ids or operation_name is required")
		}

		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, InspectGraphQLOperationOutput{}, err
		}

		sections, err := parseSections(input.Sections)
		if err != nil {
			return nil, InspectGraphQLOperationOutput{}, err
		}

		maxEntries := input.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 20
		}
		if maxEntries > 100 {
			maxEntries = 100
		}

		// Resolve entry IDs
		entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, input.EntryIDs, input.OperationName, input.Host, maxEntries)
		if err != nil {
			return nil, InspectGraphQLOperationOutput{}, err
		}

		if len(entryIDs) == 0 {
			msg := fmt.Sprintf("No entries found for operation %q. Run `survey_graphql()` to see all operation names.\n", input.OperationName)
			return textResult(msg), InspectGraphQLOperationOutput{}, nil
		}

		// Collect data across matched entries.
		// canonical captures query/schema/field stats from the first match only.
		var canonical *graphql.InspectedOperation
		entriesMatched := 0
		var errorGroups []graphql.ErrorGroup
		errSummary := graphql.ErrorSummary{}

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				continue
			}

			pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID)
			if !ok {
				continue
			}

			for _, op := range pr.Operations {
				if input.OperationName != "" && op.Name != input.OperationName {
					continue
				}

				entriesMatched++

				// Capture schema data from the first match only
				if canonical == nil && sections.needsInspect() {
					ins := graphql.InspectedOperation{ParsedOperation: op}

					if !sections.query {
						ins.RawQuery = ""
					}
					if sections.variables && op.HasVariables && op.Variables != nil {
						varBytes, _ := json.Marshal(op.Variables)
						if inferred, err := js.Infer(varBytes); err == nil && inferred != nil {
							ins.VariablesSchema = inferred.Schema
						}
					}
					if sections.responseShape {
						respBody, respCT, respErr := d.DecodeBody(entry, "response")
						if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
							if inferred, err := js.Infer(respBody); err == nil && inferred != nil {
								ins.ResponseSchema = inferred.Schema
								ins.FieldStats = js.ComputeFieldStats(inferred.Schema, [][]byte{respBody})
							}
						}
					}
					canonical = &ins
				}

				// Aggregate errors across all entries
				if sections.errors {
					errSummary.EntriesChecked++

					respBody, respCT, respErr := d.DecodeBody(entry, "response")
					if respErr != nil || respBody == nil || !contenttype.IsJSON(respCT) {
						continue
					}

					var respData struct {
						Data   any             `json:"data"`
						Errors []graphql.Error `json:"errors"`
					}
					if err := json.Unmarshal(respBody, &respData); err != nil {
						continue
					}

					if len(respData.Errors) > 0 {
						errSummary.EntriesWithErrors++
						errSummary.TotalErrors += len(respData.Errors)
						isPartial := respData.Data != nil
						if isPartial {
							errSummary.PartialFailures++
						} else {
							errSummary.FullFailures++
						}
						errorGroups = append(errorGroups, graphql.ErrorGroup{
							EntryID:       entryID,
							OperationName: op.Name,
							Errors:        respData.Errors,
							IsPartial:     isPartial,
							IsFullFailure: !isPartial,
						})
					}
				}
			}
		}

		// Determine the effective operation name
		opName := input.OperationName
		if opName == "" && canonical != nil {
			opName = canonical.Name
		} else if opName == "" && len(errorGroups) > 0 {
			opName = errorGroups[0].OperationName
		}

		// Build resource URIs
		resources := make(map[string]string)
		if opName != "" {
			resources["query"] = fmt.Sprintf("powhttp://graphql/%s/%s/query", sessionID, opName)
			resources["response_schema"] = fmt.Sprintf("powhttp://graphql/%s/%s/response-schema", sessionID, opName)
			resources["field_stats"] = fmt.Sprintf("powhttp://graphql/%s/%s/field-stats", sessionID, opName)
			resources["errors"] = fmt.Sprintf("powhttp://graphql/%s/%s/errors", sessionID, opName)
		}

		// Populate analysis cache (full data for MCP resource handlers)
		if opName != "" {
			analysis := &GraphQLAnalysis{
				SessionID:      sessionID,
				OperationName:  opName,
				EntryIDs:       entryIDs,
				EntriesMatched: entriesMatched,
				ErrorGroups:    errorGroups,
				ErrorSummary:   errSummary,
			}
			if canonical != nil {
				analysis.Query = canonical.RawQuery
				analysis.VariablesSchema = canonical.VariablesSchema
				analysis.ResponseSchema = canonical.ResponseSchema
				analysis.FieldStats = canonical.FieldStats
			}
			d.GraphQLAnalysisCache.Store(graphqlAnalysisCacheKey(sessionID, opName), analysis)
		}

		// Build output — single canonical operation, SDK serializes to JSON
		showIDs := entryIDs
		if len(showIDs) > 5 {
			showIDs = showIDs[:5]
		}

		out := InspectGraphQLOperationOutput{
			OperationName:  opName,
			EntriesMatched: entriesMatched,
			EntryIDs:       showIDs,
			Resources:      resources,
		}
		if canonical != nil {
			out.OperationType = canonical.Type
			if sections.query {
				out.Query = canonical.RawQuery
			}
			if sections.responseShape {
				out.FieldStats = canonical.FieldStats
			}
		}
		if sections.errors {
			out.ErrorGroups = errorGroups
			out.ErrorSummary = &errSummary
		}
		if len(showIDs) > 0 {
			out.Hint = fmt.Sprintf("Extract values with query_body(entry_ids=[%q], expression=\".data\").", showIDs[0])
		}

		return nil, out, nil
	}
}

// runGraphQLAnalysis runs a full analysis for a given operation (used by
// resource handlers when the cache is empty).
func runGraphQLAnalysis(ctx context.Context, d *Deps, sessionID, operationName string) (*GraphQLAnalysis, error) {
	entryIDs, err := resolveGraphQLEntryIDs(ctx, d, sessionID, nil, operationName, "", 20)
	if err != nil {
		return nil, err
	}

	analysis := &GraphQLAnalysis{
		SessionID:     sessionID,
		OperationName: operationName,
		EntryIDs:      entryIDs,
	}

	errSummary := graphql.ErrorSummary{}

	for _, entryID := range entryIDs {
		entry, err := d.FetchEntry(ctx, sessionID, entryID)
		if err != nil {
			continue
		}

		pr, ok := parseGraphQLEntry(ctx, d, sessionID, entryID)
		if !ok {
			continue
		}

		for _, op := range pr.Operations {
			if operationName != "" && op.Name != operationName {
				continue
			}
			analysis.EntriesMatched++

			// Capture query from first match
			if analysis.Query == "" {
				analysis.Query = op.RawQuery
			}

			// Variables schema from first match
			if analysis.VariablesSchema == nil && op.HasVariables && op.Variables != nil {
				varBytes, err := json.Marshal(op.Variables)
				if err == nil {
					inferred, err := js.Infer(varBytes)
					if err == nil && inferred != nil {
						analysis.VariablesSchema = inferred.Schema
					}
				}
			}

			// Response schema from first match
			if analysis.ResponseSchema == nil {
				respBody, respCT, respErr := d.DecodeBody(entry, "response")
				if respErr == nil && respBody != nil && contenttype.IsJSON(respCT) {
					inferred, err := js.Infer(respBody)
					if err == nil && inferred != nil {
						analysis.ResponseSchema = inferred.Schema
						analysis.FieldStats = js.ComputeFieldStats(inferred.Schema, [][]byte{respBody})
					}
				}
			}

			// Errors
			errSummary.EntriesChecked++
			respBody, respCT, respErr := d.DecodeBody(entry, "response")
			if respErr != nil || respBody == nil || !contenttype.IsJSON(respCT) {
				continue
			}

			var respData struct {
				Data   any             `json:"data"`
				Errors []graphql.Error `json:"errors"`
			}
			if err := json.Unmarshal(respBody, &respData); err != nil {
				continue
			}

			if len(respData.Errors) > 0 {
				errSummary.EntriesWithErrors++
				errSummary.TotalErrors += len(respData.Errors)
				isPartial := respData.Data != nil
				isFullFailure := respData.Data == nil
				if isPartial {
					errSummary.PartialFailures++
				}
				if isFullFailure {
					errSummary.FullFailures++
				}
				analysis.ErrorGroups = append(analysis.ErrorGroups, graphql.ErrorGroup{
					EntryID:       entryID,
					OperationName: op.Name,
					Errors:        respData.Errors,
					IsPartial:     isPartial,
					IsFullFailure: isFullFailure,
				})
			}
		}
	}

	analysis.ErrorSummary = errSummary

	// Cache for future requests
	d.GraphQLAnalysisCache.Store(graphqlAnalysisCacheKey(sessionID, operationName), analysis)

	return analysis, nil
}

// GetOrRunGraphQLAnalysis returns a cached analysis or runs one on demand.
// Exported for use by resource handlers in the mcp package.
func GetOrRunGraphQLAnalysis(ctx context.Context, d *Deps, sessionID, operationName string) (*GraphQLAnalysis, error) {
	key := graphqlAnalysisCacheKey(sessionID, operationName)
	if v, ok := d.GraphQLAnalysisCache.Load(key); ok {
		return v.(*GraphQLAnalysis), nil
	}
	return runGraphQLAnalysis(ctx, d, sessionID, operationName)
}
