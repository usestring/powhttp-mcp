package tools

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/shape"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// InferSchemaInput is the input for powhttp_infer_schema.
type InferSchemaInput struct {
	SessionID  string   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryIDs   []string `json:"entry_ids,omitempty" jsonschema:"Entry IDs to analyze. Obtain from search_entries, extract_endpoints, or describe_endpoint. Either entry_ids or cluster_id is required."`
	ClusterID  string   `json:"cluster_id,omitempty" jsonschema:"Cluster ID (from extract_endpoints) to analyze all entries in the cluster. Either cluster_id or entry_ids is required."`
	Target     string   `json:"target,omitempty" jsonschema:"Which body to analyze: response (default), request, or both"`
	MaxEntries int      `json:"max_entries,omitempty" jsonschema:"Max HTTP entries to inspect (default: 20, max: 100)"`
}

// ToolInferSchema infers a merged schema from multiple HTTP entry bodies.
// Dispatches to the appropriate shape analyzer based on content type:
// JSON/YAML get schema + field stats, XML gets hierarchy, CSV gets columns,
// HTML gets DOM outline, form-encoded gets key stats.
func ToolInferSchema(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input InferSchemaInput) (*sdkmcp.CallToolResult, types.InferSchemaOutput, error) {
	shapeEngine := shape.NewEngine()

	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input InferSchemaInput) (*sdkmcp.CallToolResult, types.InferSchemaOutput, error) {
		if input.ClusterID == "" && len(input.EntryIDs) == 0 {
			return nil, types.InferSchemaOutput{}, ErrInvalidInput("either cluster_id or entry_ids is required")
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
			return nil, types.InferSchemaOutput{}, ErrInvalidInput("target must be 'request', 'response', or 'both'")
		}

		// Collect entry IDs
		var entryIDs []string
		if input.ClusterID != "" {
			stored, ok := d.ClusterStore.GetCluster(input.ClusterID)
			if !ok {
				return nil, types.InferSchemaOutput{}, ErrNotFound("cluster", input.ClusterID)
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
		if maxEntries > 100 {
			maxEntries = 100
		}
		if len(entryIDs) > maxEntries {
			entryIDs = entryIDs[:maxEntries]
		}

		// Collect bodies
		var bodies [][]byte
		var detectedContentType string
		entriesProcessed := 0
		entriesSkipped := 0

		for _, entryID := range entryIDs {
			entry, err := d.FetchEntry(ctx, sessionID, entryID)
			if err != nil {
				entriesSkipped++
				continue
			}

			targets := []string{}
			if target == "request" || target == "both" {
				targets = append(targets, "request")
			}
			if target == "response" || target == "both" {
				targets = append(targets, "response")
			}

			for _, t := range targets {
				body, ct, err := d.DecodeBody(entry, t)
				if err != nil || body == nil {
					continue
				}

				if contenttype.IsBinary(ct, body) {
					continue
				}

				bodies = append(bodies, body)
				if detectedContentType == "" {
					detectedContentType = ct
				}
			}

			entriesProcessed++
		}

		if len(bodies) == 0 {
			return nil, types.InferSchemaOutput{}, ErrInvalidInput("no processable bodies found in the specified entries")
		}

		// Analyze via shape engine
		result, err := shapeEngine.Analyze(bodies, detectedContentType)
		if err != nil {
			return nil, types.InferSchemaOutput{}, fmt.Errorf("shape analysis failed: %w", err)
		}

		// Build contextual hint for tool chaining
		var hint string
		if input.ClusterID != "" {
			hint = fmt.Sprintf("Use powhttp_query_body(cluster_id=%q, expression=...) to extract specific field values based on this schema.", input.ClusterID)
		} else {
			hint = "Use powhttp_query_body(entry_ids=..., expression=...) to extract specific field values based on this schema."
		}

		output := types.InferSchemaOutput{
			Shape: result,
			Summary: types.InferSchemaSummary{
				EntriesRequested: len(entryIDs),
				EntriesProcessed: entriesProcessed,
				EntriesSkipped:   entriesSkipped,
				ContentCategory:  result.ContentCategory,
			},
			Hint: hint,
		}

		return nil, output, nil
	}
}
