package tools

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// TraceFlowInput is the input for powhttp_trace_flow.
type TraceFlowInput struct {
	SessionID   string            `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	SeedEntryID string            `json:"seed_entry_id" jsonschema:"required,Seed entry ID for tracing"`
	MaxDepth    int               `json:"max_depth,omitempty" jsonschema:"Max graph depth (default: 50)"`
	Options     *TraceFlowOptions `json:"options,omitempty" jsonschema:"Tracing options"`
	Limit       int               `json:"limit,omitempty" jsonschema:"Max nodes to return (default: 50)"`
}

// TraceFlowOptions controls flow tracing behavior.
type TraceFlowOptions struct {
	TimeWindowMs int64 `json:"time_window_ms,omitempty" jsonschema:"Time window (ms, default: 120000)"`
	SamePIDOnly  bool  `json:"same_pid_only,omitempty" jsonschema:"Same PID only (default: true)"`
	SameHostOnly bool  `json:"same_host_only,omitempty" jsonschema:"Same host only (default: true)"`
}

// TraceFlowOutput is the output for powhttp_trace_flow.
type TraceFlowOutput struct {
	Graph           *types.FlowGraph   `json:"graph"`
	EdgeTypeSummary map[string]int     `json:"edge_type_summary,omitempty"`
	Resource        *types.ResourceRef `json:"resource,omitempty"`
	Hint            string             `json:"hint,omitempty"`
}

// ToolTraceFlow traces request flow.
func ToolTraceFlow(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input TraceFlowInput) (*sdkmcp.CallToolResult, TraceFlowOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input TraceFlowInput) (*sdkmcp.CallToolResult, TraceFlowOutput, error) {
		if input.SeedEntryID == "" {
			return nil, TraceFlowOutput{}, ErrInvalidInput("seed_entry_id is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		traceReq := &types.TraceRequest{
			SessionID:   sessionID,
			SeedEntryID: input.SeedEntryID,
			MaxDepth:    input.MaxDepth,
			Limit:       input.Limit,
		}

		if input.Options != nil {
			traceReq.Options = &types.TraceOptions{
				TimeWindowMs: input.Options.TimeWindowMs,
				SamePIDOnly:  input.Options.SamePIDOnly,
				SameHostOnly: input.Options.SameHostOnly,
			}
		}

		graph, err := d.Flow.Trace(ctx, traceReq)
		if err != nil {
			return nil, TraceFlowOutput{}, WrapPowHTTPError(err)
		}

		// Compute edge type summary
		var edgeSummary map[string]int
		if graph != nil && len(graph.Edges) > 0 {
			edgeSummary = make(map[string]int)
			for _, edge := range graph.Edges {
				edgeSummary[edge.Reason]++
			}
		}

		// Build helpful hint based on graph structure
		var hint string
		if graph == nil || len(graph.Nodes) == 0 {
			hint = "No related requests. Try: same_host_only=false, same_pid_only=false, or increase time_window_ms."
		} else if len(graph.Edges) == 0 {
			hint = "Single isolated request. Try same_pid_only=false or same_host_only=false to find related traffic."
		} else {
			hint = fmt.Sprintf("Found %d related requests. Use get_entry to inspect individual nodes.", len(graph.Nodes))
		}

		return nil, TraceFlowOutput{
			Graph:           graph,
			EdgeTypeSummary: edgeSummary,
			Resource: &types.ResourceRef{
				URI:  "powhttp://flow/" + input.SeedEntryID,
				MIME: MimeJSON,
				Hint: "Fetch for raw flow graph data export",
			},
			Hint: hint,
		}, nil
	}
}
