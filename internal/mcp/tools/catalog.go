package tools

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ExtractEndpointsInput is the input for powhttp_extract_endpoints.
type ExtractEndpointsInput struct {
	SessionID string                    `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Scope     *ExtractEndpointsScope    `json:"scope,omitempty" jsonschema:"Pre-clustering filters (narrows input entries)"`
	Filters   *ExtractEndpointsFilters  `json:"filters,omitempty" jsonschema:"Post-clustering filters (narrows output clusters)"`
	Options   *ExtractEndpointsOptions  `json:"options,omitempty" jsonschema:"Clustering options"`
	Limit     int                       `json:"limit,omitempty" jsonschema:"Max clusters to return (default: 50)"`
	Offset    int                       `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

// ExtractEndpointsScope defines pre-clustering filters that narrow input entries.
type ExtractEndpointsScope struct {
	Host         string `json:"host,omitempty" jsonschema:"Filter by host. Prefix with '*.' to include subdomains: '*.example.com' matches example.com, api.example.com, etc. Prefer '*.domain' to capture all related traffic."`
	Method       string `json:"method,omitempty" jsonschema:"Filter by HTTP method (e.g., GET, POST). Case-insensitive; applied before clustering."`
	ProcessName  string `json:"process_name,omitempty" jsonschema:"Filter by process name"`
	PID          int    `json:"pid,omitempty" jsonschema:"Filter by process ID"`
	TimeWindowMs int64  `json:"time_window_ms,omitempty" jsonschema:"Relative time window (ms)"`
	SinceMs      int64  `json:"since_ms,omitempty" jsonschema:"Unix timestamp (ms) lower bound"`
	UntilMs      int64  `json:"until_ms,omitempty" jsonschema:"Unix timestamp (ms) upper bound"`
}

// ExtractEndpointsFilters defines post-clustering filters that narrow output clusters.
type ExtractEndpointsFilters struct {
	Category string `json:"category,omitempty" jsonschema:"Filter clusters by endpoint category. Valid values: api, page, asset, data, other"`
	MinCount int    `json:"min_count,omitempty" jsonschema:"Minimum requests per cluster (filters out low-traffic endpoints)"`
}

// ExtractEndpointsOptions controls clustering behavior.
type ExtractEndpointsOptions struct {
	NormalizeIDs           bool `json:"normalize_ids,omitempty" jsonschema:"Normalize path IDs (default: true)"`
	StripVolatileQueryKeys bool `json:"strip_volatile_query_keys,omitempty" jsonschema:"Strip volatile query keys (default: true)"`
	ExamplesPerCluster     int  `json:"examples_per_cluster,omitempty" jsonschema:"Examples per cluster (default: 3)"`
	MaxClusters            int  `json:"max_clusters,omitempty" jsonschema:"Max clusters (default: 200)"`
}

// ExtractEndpointsOutput is the output for powhttp_extract_endpoints.
type ExtractEndpointsOutput struct {
	Clusters   []types.Cluster    `json:"clusters,omitzero"`
	TotalCount int                `json:"total_count"`
	ScopeHash  string             `json:"scope_hash"`
	Resource   *types.ResourceRef `json:"resource,omitempty"`
	Hint       string             `json:"hint,omitempty"`
}

// DescribeEndpointInput is the input for powhttp_describe_endpoint.
type DescribeEndpointInput struct {
	SessionID   string `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	ClusterID   string `json:"cluster_id" jsonschema:"required,Cluster ID from extract_endpoints"`
	MaxExamples int    `json:"max_examples,omitempty" jsonschema:"Max example entries (default: 5)"`
}

// DescribeEndpointOutput is the output for powhttp_describe_endpoint.
type DescribeEndpointOutput struct {
	Description *types.EndpointDescription `json:"description"`
	Resource    *types.ResourceRef         `json:"resource,omitempty"`
	Hint        string                     `json:"hint,omitempty"`
}

// validCategories lists the accepted category filter values.
var validCategories = map[string]bool{
	"api": true, "page": true, "asset": true, "data": true, "other": true,
}

// ToolExtractEndpoints extracts endpoint clusters.
func ToolExtractEndpoints(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ExtractEndpointsInput) (*sdkmcp.CallToolResult, ExtractEndpointsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ExtractEndpointsInput) (*sdkmcp.CallToolResult, ExtractEndpointsOutput, error) {
		// Validate category filter
		if input.Filters != nil && input.Filters.Category != "" {
			if !validCategories[input.Filters.Category] {
				return nil, ExtractEndpointsOutput{}, ErrInvalidInput(
					fmt.Sprintf("invalid category %q, must be one of: api, page, asset, data, other", input.Filters.Category))
			}
		}

		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, ExtractEndpointsOutput{}, err
		}

		limit := input.Limit
		if limit <= 0 {
			limit = d.Config.DefaultClusterLimit
		}

		extractReq := &types.ExtractRequest{
			SessionID: sessionID,
			Limit:     limit,
			Offset:    input.Offset,
		}

		if input.Scope != nil {
			extractReq.Scope = &types.ClusterScope{
				Host:         input.Scope.Host,
				Method:       input.Scope.Method,
				ProcessName:  input.Scope.ProcessName,
				PID:          input.Scope.PID,
				TimeWindowMs: input.Scope.TimeWindowMs,
				SinceMs:      input.Scope.SinceMs,
				UntilMs:      input.Scope.UntilMs,
			}
		}

		if input.Filters != nil {
			extractReq.Filters = &types.ClusterFilters{
				Category: types.EndpointCategory(input.Filters.Category),
				MinCount: input.Filters.MinCount,
			}
		}

		if input.Options != nil {
			extractReq.Options = &types.ClusterOptions{
				NormalizeIDs:           input.Options.NormalizeIDs,
				StripVolatileQueryKeys: input.Options.StripVolatileQueryKeys,
				ExamplesPerCluster:     input.Options.ExamplesPerCluster,
				MaxClusters:            input.Options.MaxClusters,
			}
		}

		resp, err := d.Cluster.Extract(ctx, extractReq)
		if err != nil {
			return nil, ExtractEndpointsOutput{}, WrapPowHTTPError(err)
		}

		// Build helpful hint with concrete values
		var hintParts []string
		if len(resp.Clusters) == 0 {
			hintParts = append(hintParts, "No endpoints found. Session may be empty or filters too restrictive.")
		} else if resp.TotalCount > len(resp.Clusters) {
			nextOffset := input.Offset + len(resp.Clusters)
			hintParts = append(hintParts, fmt.Sprintf("Showing %d of %d clusters. Use scope.host to filter, or offset=%d for next page.", len(resp.Clusters), resp.TotalCount, nextOffset))
		} else {
			hintParts = append(hintParts, fmt.Sprintf("Found %d clusters. Use powhttp_describe_endpoint(cluster_id=...) for schema and examples.", len(resp.Clusters)))
		}

		// Category breakdown when diverse
		if len(resp.Clusters) > 0 {
			catCounts := make(map[types.EndpointCategory]int)
			for _, c := range resp.Clusters {
				catCounts[c.Category]++
			}
			if len(catCounts) > 1 {
				hintParts = append(hintParts, fmt.Sprintf("Categories: %s.", formatCategoryCounts(catCounts)))
				hintParts = append(hintParts, "Use filters.category to focus (e.g., filters={category: \"api\"}).")
			}
		}

		// Detect GraphQL endpoints by probing a sample entry body from each
		// POST cluster. No path pre-filtering — catches custom paths like /api/data.
		// Uses the shared parse cache so results are reusable across tool calls.
		var gqlEndpoints []string // display labels, e.g. "POST api.example.com/graphql (45 reqs)"
		var gqlHosts []string    // corresponding host for each endpoint
		for _, c := range resp.Clusters {
			if c.Method != "POST" || len(c.ExampleEntryIDs) == 0 {
				continue
			}
			// Try all example entries — any single valid GraphQL body confirms the cluster.
			isGQL := false
			for _, eid := range c.ExampleEntryIDs {
				if _, ok := parseGraphQLEntry(ctx, d, sessionID, eid); ok {
					isGQL = true
					break
				}
			}
			if !isGQL {
				continue
			}
			gqlEndpoints = append(gqlEndpoints, fmt.Sprintf("POST %s%s (%d reqs)", c.Host, c.PathTemplate, c.Count))
			gqlHosts = append(gqlHosts, c.Host)
		}

		if len(gqlEndpoints) > 0 {
			hintParts = append(hintParts, fmt.Sprintf("GraphQL detected: %s.", strings.Join(gqlEndpoints, "; ")))
			if len(gqlEndpoints) == 1 {
				hintParts = append(hintParts, fmt.Sprintf("Use powhttp_graphql_operations(scope={host: %q}) for operation-level analysis.", gqlHosts[0]))
			} else {
				hintParts = append(hintParts, "Use powhttp_graphql_operations() for operation-level analysis.")
			}
		}

		return nil, ExtractEndpointsOutput{
			Clusters:   resp.Clusters,
			TotalCount: resp.TotalCount,
			ScopeHash:  resp.ScopeHash,
			Resource: &types.ResourceRef{
				URI:  "powhttp://catalog/" + resp.ScopeHash,
				MIME: MimeJSON,
				Hint: "Fetch for complete endpoint catalog dump",
			},
			Hint: strings.Join(hintParts, " "),
		}, nil
	}
}

// formatCategoryCounts formats category counts as a compact string.
func formatCategoryCounts(counts map[types.EndpointCategory]int) string {
	order := []types.EndpointCategory{
		types.CategoryAPI, types.CategoryPage, types.CategoryAsset,
		types.CategoryData, types.CategoryOther,
	}
	var parts []string
	for _, cat := range order {
		if n, ok := counts[cat]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, cat))
		}
	}
	return strings.Join(parts, ", ")
}

// ToolDescribeEndpoint describes an endpoint cluster.
func ToolDescribeEndpoint(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input DescribeEndpointInput) (*sdkmcp.CallToolResult, DescribeEndpointOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input DescribeEndpointInput) (*sdkmcp.CallToolResult, DescribeEndpointOutput, error) {
		if input.ClusterID == "" {
			return nil, DescribeEndpointOutput{}, ErrInvalidInput("cluster_id is required")
		}

		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, DescribeEndpointOutput{}, err
		}

		descReq := &types.DescribeRequest{
			ClusterID:   input.ClusterID,
			SessionID:   sessionID,
			MaxExamples: input.MaxExamples,
		}

		desc, err := d.Describe.Describe(ctx, descReq)
		if err != nil {
			return nil, DescribeEndpointOutput{}, WrapPowHTTPError(err)
		}

		// Build contextual hint
		hint := fmt.Sprintf("Use powhttp_query_body(cluster_id=%q, expression='.') to explore response structure, or powhttp_infer_schema(cluster_id=%q) for deeper field statistics.", input.ClusterID, input.ClusterID)

		return nil, DescribeEndpointOutput{
			Description: desc,
			Hint:        hint,
		}, nil
	}
}

