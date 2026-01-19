package tools

import (
	"context"
	"encoding/base64"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/query"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// QueryBodyInput is the input for powhttp_query_body.
type QueryBodyInput struct {
	SessionID   string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs    []string `json:"entry_ids,omitempty" jsonschema:"Query these specific entry IDs"`
	ClusterID   string   `json:"cluster_id,omitempty" jsonschema:"Query all entries in this cluster"`
	Expression  string   `json:"expression" jsonschema:"required,JQ expression (e.g., '.data.items[].name')"`
	Target      string   `json:"target,omitempty" jsonschema:"Which body to query: request, response, or both (default: response)"`
	Deduplicate bool     `json:"deduplicate,omitempty" jsonschema:"Remove duplicate values (default: false)"`
	MaxEntries  int      `json:"max_entries,omitempty" jsonschema:"Max entries to process (default: 20, max: 100)"`
	MaxResults  int      `json:"max_results,omitempty" jsonschema:"Max results to return (default: 1000)"`
}

// ToolQueryBody queries HTTP entry bodies using JQ expressions.
func ToolQueryBody(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input QueryBodyInput) (*sdkmcp.CallToolResult, types.QueryResponse, error) {
	engine := query.NewEngine()

	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input QueryBodyInput) (*sdkmcp.CallToolResult, types.QueryResponse, error) {
		// Validate required input
		if input.Expression == "" {
			return nil, types.QueryResponse{}, ErrInvalidInput("expression is required")
		}

		if input.ClusterID == "" && len(input.EntryIDs) == 0 {
			return nil, types.QueryResponse{}, ErrInvalidInput("either cluster_id or entry_ids is required")
		}

		// Validate expression first
		if err := engine.ValidateExpression(input.Expression); err != nil {
			return nil, types.QueryResponse{}, ErrInvalidInput(err.Error())
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
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
			maxEntries = 20
		}
		if maxEntries > 100 {
			maxEntries = 100
		}

		maxResults := input.MaxResults
		if maxResults <= 0 {
			maxResults = 1000
		}

		truncatedEntries := len(entryIDs) > maxEntries
		if truncatedEntries {
			entryIDs = entryIDs[:maxEntries]
		}

		// Collect bodies and process
		output := types.QueryResponse{
			Summary: types.QuerySummary{
				Deduplicated: input.Deduplicate,
			},
			Values:  make([]any, 0),
			Entries: make([]types.QueryEntryResult, 0),
			Errors:  make([]string, 0),
			Hints:   make([]string, 0),
		}

		var allBodies [][]byte
		var bodyLabels []string // Labels for error messages (entry_id:target)

		for _, entryID := range entryIDs {
			entry, err := fetchEntry(ctx, d, sessionID, entryID)
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

			// Process request body
			if target == "request" || target == "both" {
				bodyBytes, skip, reason := extractBody(entry, "request")
				if skip {
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     "request",
						Skipped:    true,
						SkipReason: reason,
					})
				} else if bodyBytes != nil {
					allBodies = append(allBodies, bodyBytes)
					bodyLabels = append(bodyLabels, entryID+":request")
				}
			}

			// Process response body
			if target == "response" || target == "both" {
				bodyBytes, skip, reason := extractBody(entry, "response")
				if skip {
					output.Entries = append(output.Entries, types.QueryEntryResult{
						EntryID:    entryID,
						Target:     "response",
						Skipped:    true,
						SkipReason: reason,
					})
				} else if bodyBytes != nil {
					allBodies = append(allBodies, bodyBytes)
					bodyLabels = append(bodyLabels, entryID+":response")
				}
			}
		}

		// Run the query across all bodies
		if len(allBodies) > 0 {
			result, err := engine.QueryMultipleWithLabels(allBodies, bodyLabels, input.Expression, input.Deduplicate, maxResults)
			if err != nil {
				return nil, types.QueryResponse{}, ErrInvalidInput(err.Error())
			}

			output.Values = result.Values
			output.Errors = append(output.Errors, result.Errors...)
			output.Summary.TotalValues = result.RawCount
			output.Summary.UniqueValues = len(result.Values)
			output.Summary.EntriesMatched = countMatchedBodies(allBodies, result.RawCount)
			output.Summary.Truncated = len(result.Values) >= maxResults || truncatedEntries
		}

		// Add helpful hints
		if output.Summary.TotalValues == 0 && len(allBodies) > 0 {
			output.Hints = append(output.Hints, "No values matched. Try a simpler expression like '.' to see the full structure.")
		}
		if input.Deduplicate && output.Summary.TotalValues > output.Summary.UniqueValues {
			output.Hints = append(output.Hints, "Deduplication removed "+string(rune(output.Summary.TotalValues-output.Summary.UniqueValues+'0'))+" duplicate values.")
		}
		if output.Summary.Truncated {
			output.Hints = append(output.Hints, "Results were truncated. Use filters or increase max_results.")
		}

		return nil, output, nil
	}
}

// extractBody extracts and decodes the body from an entry for the given target.
// Returns (bodyBytes, skipped, skipReason).
func extractBody(entry *client.SessionEntry, target string) ([]byte, bool, string) {
	var body *string
	var contentType string

	if target == "request" {
		body = entry.Request.Body
		contentType = entry.Request.Headers.Get("content-type")
	} else {
		if entry.Response == nil {
			return nil, true, "no response"
		}
		body = entry.Response.Body
		contentType = entry.Response.Headers.Get("content-type")
	}

	if body == nil || *body == "" {
		return nil, true, "no body"
	}

	// Check for JSON content type
	if !strings.Contains(strings.ToLower(contentType), "json") {
		return nil, true, "not JSON content-type: " + contentType
	}

	bodyBytes, err := base64.StdEncoding.DecodeString(*body)
	if err != nil {
		return nil, true, "failed to decode body: " + err.Error()
	}

	return bodyBytes, false, ""
}

// countMatchedBodies estimates how many bodies had matches based on total values.
func countMatchedBodies(bodies [][]byte, totalValues int) int {
	if totalValues == 0 {
		return 0
	}
	// Simple estimation - at least 1, at most all bodies
	return min(totalValues, len(bodies))
}
