package tools

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ExtractEndpointsInput is the input for powhttp_extract_endpoints.
type ExtractEndpointsInput struct {
	SessionID string                   `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Scope     *ExtractEndpointsScope   `json:"scope,omitempty" jsonschema:"Filtering scope"`
	Options   *ExtractEndpointsOptions `json:"options,omitempty" jsonschema:"Clustering options"`
	Limit     int                      `json:"limit,omitempty" jsonschema:"Max clusters to return (default: 50)"`
	Offset    int                      `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

// ExtractEndpointsScope defines the filtering scope.
type ExtractEndpointsScope struct {
	Host         string `json:"host,omitempty" jsonschema:"Filter by host"`
	ProcessName  string `json:"process_name,omitempty" jsonschema:"Filter by process name"`
	PID          int    `json:"pid,omitempty" jsonschema:"Filter by process ID"`
	TimeWindowMs int64  `json:"time_window_ms,omitempty" jsonschema:"Relative time window (ms)"`
	SinceMs      int64  `json:"since_ms,omitempty" jsonschema:"Unix timestamp (ms) lower bound"`
	UntilMs      int64  `json:"until_ms,omitempty" jsonschema:"Unix timestamp (ms) upper bound"`
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
	Clusters   []types.Cluster    `json:"clusters"`
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

// ToolExtractEndpoints extracts endpoint clusters.
func ToolExtractEndpoints(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ExtractEndpointsInput) (*sdkmcp.CallToolResult, ExtractEndpointsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ExtractEndpointsInput) (*sdkmcp.CallToolResult, ExtractEndpointsOutput, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
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
				ProcessName:  input.Scope.ProcessName,
				PID:          input.Scope.PID,
				TimeWindowMs: input.Scope.TimeWindowMs,
				SinceMs:      input.Scope.SinceMs,
				UntilMs:      input.Scope.UntilMs,
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
		var hint string
		if len(resp.Clusters) == 0 {
			hint = "No endpoints found. Session may be empty or scope filters too restrictive."
		} else if resp.TotalCount > len(resp.Clusters) {
			nextOffset := input.Offset + len(resp.Clusters)
			hint = fmt.Sprintf("Showing %d of %d clusters. Use scope.host to filter, or offset=%d for next page.", len(resp.Clusters), resp.TotalCount, nextOffset)
		} else {
			hint = fmt.Sprintf("Found %d clusters. Use powhttp_describe_endpoint(cluster_id=...) for schema and examples.", len(resp.Clusters))
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
			Hint: hint,
		}, nil
	}
}

// ToolDescribeEndpoint describes an endpoint cluster.
func ToolDescribeEndpoint(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input DescribeEndpointInput) (*sdkmcp.CallToolResult, DescribeEndpointOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input DescribeEndpointInput) (*sdkmcp.CallToolResult, DescribeEndpointOutput, error) {
		if input.ClusterID == "" {
			return nil, DescribeEndpointOutput{}, ErrInvalidInput("cluster_id is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
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
