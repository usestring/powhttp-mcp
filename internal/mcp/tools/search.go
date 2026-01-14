package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// SearchEntriesInput is the input for powhttp_search_entries.
type SearchEntriesInput struct {
	SessionID string                `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Query     string                `json:"query,omitempty" jsonschema:"Free text search query"`
	Filters   *SearchEntriesFilters `json:"filters,omitempty" jsonschema:"Structured filters"`
	Limit     int                   `json:"limit,omitempty" jsonschema:"Max results (default: 20, max: 100)"`
	Offset    int                   `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

// SearchEntriesFilters contains filter criteria for search.
type SearchEntriesFilters struct {
	Host            string `json:"host,omitempty" jsonschema:"Filter by host"`
	PathContains    string `json:"path_contains,omitempty" jsonschema:"Path substring match"`
	URLContains     string `json:"url_contains,omitempty" jsonschema:"URL substring match"`
	Method          string `json:"method,omitempty" jsonschema:"HTTP method"`
	Status          int    `json:"status,omitempty" jsonschema:"HTTP status code"`
	HTTPVersion     string `json:"http_version,omitempty" jsonschema:"HTTP version"`
	ProcessName     string `json:"process_name,omitempty" jsonschema:"Process name"`
	PID             int    `json:"pid,omitempty" jsonschema:"Process ID"`
	HeaderName      string `json:"header_name,omitempty" jsonschema:"Has header name"`
	TLSConnectionID string `json:"tls_connection_id,omitempty" jsonschema:"TLS connection ID"`
	JA3             string `json:"ja3,omitempty" jsonschema:"JA3 fingerprint hash"`
	JA4             string `json:"ja4,omitempty" jsonschema:"JA4 fingerprint hash"`
	SinceMs         int64  `json:"since_ms,omitempty" jsonschema:"Unix timestamp (ms) lower bound"`
	UntilMs         int64  `json:"until_ms,omitempty" jsonschema:"Unix timestamp (ms) upper bound"`
	TimeWindowMs    int64  `json:"time_window_ms,omitempty" jsonschema:"Relative time window (ms from now)"`
}

// SearchEntriesOutput is the output for powhttp_search_entries.
type SearchEntriesOutput struct {
	Results    []types.SearchResult `json:"results"`
	TotalHint  int                  `json:"total_hint,omitempty"`
	SyncedAtMs int64                `json:"synced_at_ms"`
}

// ToolSearchEntries searches HTTP entries.
func ToolSearchEntries(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input SearchEntriesInput) (*sdkmcp.CallToolResult, SearchEntriesOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input SearchEntriesInput) (*sdkmcp.CallToolResult, SearchEntriesOutput, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		searchReq := &types.SearchRequest{
			SessionID: sessionID,
			Query:     input.Query,
			Limit:     input.Limit,
			Offset:    input.Offset,
		}

		if input.Filters != nil {
			searchReq.Filters = &types.SearchFilters{
				Host:            input.Filters.Host,
				PathContains:    input.Filters.PathContains,
				URLContains:     input.Filters.URLContains,
				Method:          input.Filters.Method,
				Status:          input.Filters.Status,
				HTTPVersion:     input.Filters.HTTPVersion,
				ProcessName:     input.Filters.ProcessName,
				PID:             input.Filters.PID,
				HeaderName:      input.Filters.HeaderName,
				TLSConnectionID: input.Filters.TLSConnectionID,
				JA3:             input.Filters.JA3,
				JA4:             input.Filters.JA4,
				SinceMs:         input.Filters.SinceMs,
				UntilMs:         input.Filters.UntilMs,
				TimeWindowMs:    input.Filters.TimeWindowMs,
			}
		}

		resp, err := d.Search.Search(ctx, searchReq)
		if err != nil {
			return nil, SearchEntriesOutput{}, WrapPowHTTPError(err)
		}

		return nil, SearchEntriesOutput{
			Results:    resp.Results,
			TotalHint:  resp.TotalHint,
			SyncedAtMs: resp.SyncedAtMs,
		}, nil
	}
}
