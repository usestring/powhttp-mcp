package tools

import (
	"context"
	"fmt"
	"log/slog"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/textquery"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// QueryBodyInput is the input for powhttp_query_body.
type QueryBodyInput struct {
	SessionID   string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs    []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to query (from search_entries results). Either entry_ids or cluster_id is required."`
	ClusterID   string   `json:"cluster_id,omitempty" jsonschema:"Cluster ID to query all its entries (from extract_endpoints). Either cluster_id or entry_ids is required."`
	Expression  string   `json:"expression" jsonschema:"required,Extraction expression matching the body content type. Examples -- JQ: '.data.items[].name'; CSS: 'h1.title'; XPath: '//item/name'; regex: 'token=([\\\\w]+)'; form: 'email' or '*' for all keys"`
	Mode        string   `json:"mode,omitempty" jsonschema:"Override auto-detection with an explicit expression language. Valid values: jq, css, xpath, regex, form. Omit to auto-detect from content-type (recommended)."`
	Target      string   `json:"target,omitempty" jsonschema:"Which body to query: response (default), request, or both"`
	Deduplicate bool     `json:"deduplicate,omitempty" jsonschema:"Remove duplicate values (default: false)"`
	MaxEntries  int      `json:"max_entries,omitempty" jsonschema:"Max HTTP entries to inspect (default: 20, max: 100). Limits how many entries from entry_ids or cluster are processed."`
	MaxResults  int      `json:"max_results,omitempty" jsonschema:"Max extracted values to return across all entries (default: 1000). Increase if results are truncated."`
}

// ToolQueryBody extracts data from request/response bodies using expressions.
// The expression language is auto-detected from content-type (JQ for JSON/YAML,
// CSS selectors for HTML, XPath for XML, regex for plain text, form key for
// form-encoded) or can be set explicitly via the mode parameter.
func ToolQueryBody(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input QueryBodyInput) (*sdkmcp.CallToolResult, types.QueryResponse, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input QueryBodyInput) (*sdkmcp.CallToolResult, types.QueryResponse, error) {
		if input.Expression == "" {
			return nil, types.QueryResponse{}, ErrInvalidInput("expression is required")
		}
		if input.ClusterID == "" && len(input.EntryIDs) == 0 {
			return nil, types.QueryResponse{}, ErrInvalidInput("either cluster_id or entry_ids is required")
		}

		// Validate expression if mode is explicitly set
		if input.Mode != "" {
			if err := d.TextQuery.ValidateExpression(input.Expression, input.Mode); err != nil {
				return nil, types.QueryResponse{}, ErrInvalidInput(err.Error())
			}
		}

		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, types.QueryResponse{}, err
		}

		target := input.Target
		if target == "" {
			target = "response"
		}
		if target != "request" && target != "response" && target != "both" {
			return nil, types.QueryResponse{}, ErrInvalidInput("target must be 'request', 'response', or 'both'")
		}

		// Collect entry IDs
		var entryIDs []string
		if input.ClusterID != "" {
			stored, ok := d.ClusterStore.GetCluster(input.ClusterID)
			if !ok {
				return nil, types.QueryResponse{}, ErrNotFound("cluster", input.ClusterID)
			}
			entryIDs = stored.EntryIDs
		} else {
			entryIDs = input.EntryIDs
		}

		// Apply limits
		maxEntries := input.MaxEntries
		if maxEntries <= 0 {
			maxEntries = d.Config.DefaultQueryLimit
		}
		if cap := d.Config.MaxQueryEntries; maxEntries > cap {
			slog.Warn("query_body max_entries capped", "requested", maxEntries, "cap", cap)
			maxEntries = cap
		}

		maxResults := input.MaxResults
		if maxResults <= 0 {
			maxResults = 1000
		}

		originalEntryCount := len(entryIDs)
		truncatedEntries := originalEntryCount > maxEntries
		if truncatedEntries {
			entryIDs = entryIDs[:maxEntries]
		}

		output := types.QueryResponse{
			Summary: types.QuerySummary{
				Deduplicated: input.Deduplicate,
			},
			Values:  make([]any, 0),
			Entries: make([]types.QueryEntryResult, 0),
			Errors:  make([]string, 0),
			Hints:   make([]string, 0),
		}

		seen := make(map[string]bool)
		var lastMode string

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				output.Entries = append(output.Entries, types.QueryEntryResult{
					EntryID:    entryID,
					Target:     target,
					Skipped:    true,
					SkipReason: "failed to fetch entry: " + err.Error(),
				})
				output.Summary.EntriesSkipped++
				continue
			}

			output.Summary.EntriesProcessed++

			targets := []string{}
			if target == "request" || target == "both" {
				targets = append(targets, "request")
			}
			if target == "response" || target == "both" {
				targets = append(targets, "response")
			}

			for _, t := range targets {
				body, ct, err := d.DecodeBody(entry, t)
				if err != nil {
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     t,
						Skipped:    true,
						SkipReason: "failed to decode body: " + err.Error(),
					})
					continue
				}

				if body == nil {
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     t,
						Skipped:    true,
						SkipReason: "no body",
					})
					continue
				}

				if contenttype.IsBinary(ct, body) {
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     t,
						Skipped:    true,
						SkipReason: "binary content type: " + ct,
					})
					continue
				}

				result, err := d.TextQuery.Query(body, ct, input.Expression, input.Mode, maxResults)
				if err != nil {
					output.Errors = append(output.Errors, fmt.Sprintf("%s:%s: %s", entryID, t, err.Error()))
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     t,
						Skipped:    true,
						SkipReason: err.Error(),
					})
					continue
				}

				lastMode = result.Mode

				// Propagate runtime warnings (e.g., JQ evaluation errors)
				for _, e := range result.Errors {
					output.Errors = append(output.Errors, fmt.Sprintf("%s:%s: %s", entryID, t, e))
				}

				entryResult := types.QueryEntryResult{
					EntryID:    entryID,
					Target:     t,
					ValueCount: result.Count,
				}

				for _, v := range result.Values {
					if maxResults > 0 && len(output.Values) >= maxResults {
						break
					}
					if input.Deduplicate {
						key := fmt.Sprintf("%v", v)
						if seen[key] {
							continue
						}
						seen[key] = true
					}
					output.Values = append(output.Values, v)
				}

				if result.Count > 0 {
					output.Summary.EntriesMatched++
				}
				output.Entries = append(output.Entries, entryResult)
			}
		}

		output.Summary.TotalValues = len(output.Values)
		output.Summary.UniqueValues = len(output.Values)
		output.Summary.Truncated = len(output.Values) >= maxResults || truncatedEntries

		// Hints
		if output.Summary.TotalValues == 0 && output.Summary.EntriesProcessed > 0 {
			hint := "No values extracted."
			if lastMode != "" {
				hint += fmt.Sprintf(" Auto-detected mode: %s.", lastMode)
			}
			switch lastMode {
			case textquery.ModeJQ:
				hint += " Try '.', 'keys', or '.data.items[]' to explore the structure."
			case textquery.ModeCSS:
				hint += " Try '*' to match all elements, or check the selector syntax."
			case textquery.ModeXPath:
				hint += " Try '//*' to match all elements, or check the expression."
			default:
				hint += " Try a broader expression or set mode explicitly if auto-detection chose wrong."
			}
			output.Hints = append(output.Hints, hint)
		}
		if output.Summary.Truncated {
			if truncatedEntries && len(output.Values) < maxResults {
				output.Hints = append(output.Hints, fmt.Sprintf(
					"Processed %d of %d entries (max_entries=%d). Increase max_entries or use entry_ids to target specific entries.",
					maxEntries, originalEntryCount, maxEntries))
			} else if len(output.Values) >= maxResults {
				nextMax := maxResults * 2
				if nextMax > 10000 {
					nextMax = 10000
				}
				output.Hints = append(output.Hints, fmt.Sprintf(
					"Value limit reached (%d). Narrow with entry_ids or increase max_results=%d.",
					maxResults, nextMax))
			}
		}
		if len(output.Errors) > 0 && output.Summary.TotalValues > 0 {
			output.Hints = append(output.Hints, fmt.Sprintf(
				"Partial results: %d entries had errors. Check the errors array for details.",
				output.Summary.EntriesSkipped))
		}

		return nil, output, nil
	}
}
