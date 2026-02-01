package tools

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// SearchEntriesInput is the input for powhttp_search_entries.
type SearchEntriesInput struct {
	SessionID      string                `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	Query          string                `json:"query,omitempty" jsonschema:"Free text search across URLs, query params, headers, and body content. Tokens are ANDed: all terms must match somewhere. Use for broad discovery."`
	Filters        *SearchEntriesFilters `json:"filters,omitempty" jsonschema:"Structured filters"`
	Limit          int                   `json:"limit,omitempty" jsonschema:"Max results (default: 10, max: 100)"`
	Offset         int                   `json:"offset,omitempty" jsonschema:"Pagination offset"`
	IncludeDetails bool                  `json:"include_details,omitempty" jsonschema:"Include full details (TLS, HTTP2, Sizes). Default: false"`
}

// SearchEntriesFilters contains filter criteria for search.
type SearchEntriesFilters struct {
	Host            string `json:"host,omitempty" jsonschema:"Filter by host. Prefix with '*.' to include subdomains: '*.example.com' matches example.com, api.example.com, etc. Prefer '*.domain' to capture all related traffic."`
	PathContains    string `json:"path_contains,omitempty" jsonschema:"Path substring match"`
	URLContains     string `json:"url_contains,omitempty" jsonschema:"URL substring match"`
	Method          string `json:"method,omitempty" jsonschema:"HTTP method"`
	Status          int    `json:"status,omitempty" jsonschema:"HTTP status code"`
	HTTPVersion     string `json:"http_version,omitempty" jsonschema:"HTTP version"`
	ProcessName     string `json:"process_name,omitempty" jsonschema:"Process name"`
	PID             int    `json:"pid,omitempty" jsonschema:"Process ID"`
	HeaderName      string `json:"header_name,omitempty" jsonschema:"Filter by header presence (name only, e.g., authorization)"`
	HeaderContains  string `json:"header_contains,omitempty" jsonschema:"Substring match on header fields (searches name and value, e.g., 'bearer' or 'content-type: json')"`
	BodyContains    string `json:"body_contains,omitempty" jsonschema:"Substring match on decoded request/response body text. Searches cached entries only."`
	TLSConnectionID string `json:"tls_connection_id,omitempty" jsonschema:"TLS connection ID"`
	JA3             string `json:"ja3,omitempty" jsonschema:"JA3 fingerprint hash"`
	JA4             string `json:"ja4,omitempty" jsonschema:"JA4 fingerprint hash"`
	SinceMs         int64  `json:"since_ms,omitempty" jsonschema:"Unix timestamp (ms) lower bound"`
	UntilMs         int64  `json:"until_ms,omitempty" jsonschema:"Unix timestamp (ms) upper bound"`
	TimeWindowMs    int64  `json:"time_window_ms,omitempty" jsonschema:"Relative time window (ms from now)"`
}

// SearchEntriesOutput is the output for powhttp_search_entries.
type SearchEntriesOutput struct {
	Results       []types.SearchResult `json:"results,omitzero"`
	TotalHint     int                  `json:"total_hint,omitempty"`
	SyncedAtMs    int64                `json:"synced_at_ms"`
	SearchedScope *types.SearchScope   `json:"searched_scope,omitempty"`
	Hint          string               `json:"hint,omitempty"`
}

// ToolSearchEntries searches HTTP entries.
func ToolSearchEntries(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input SearchEntriesInput) (*sdkmcp.CallToolResult, SearchEntriesOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input SearchEntriesInput) (*sdkmcp.CallToolResult, SearchEntriesOutput, error) {
		sessionID, err := d.ResolveSessionID(ctx, input.SessionID)
		if err != nil {
			return nil, SearchEntriesOutput{}, err
		}

		limit := input.Limit
		if limit <= 0 {
			limit = d.Config.DefaultSearchLimit
		}

		searchReq := &types.SearchRequest{
			SessionID: sessionID,
			Query:     input.Query,
			Limit:     limit,
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
				HeaderContains:  input.Filters.HeaderContains,
				BodyContains:    input.Filters.BodyContains,
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

		// Thin out results if details not requested - keep only essential fields
		if !input.IncludeDetails {
			for i := range resp.Results {
				if resp.Results[i].Summary != nil {
					// Keep content type hint but zero out heavy fields
					contentType := resp.Results[i].Summary.Sizes.RespContentType
					resp.Results[i].Summary.TLS = types.TLSSummary{}
					resp.Results[i].Summary.HTTP2 = types.HTTP2Summary{}
					resp.Results[i].Summary.Sizes = types.SizeSummary{RespContentType: contentType}
					// Zero out process info (can be retrieved via get_entry if needed)
					resp.Results[i].Summary.ProcessName = ""
					resp.Results[i].Summary.PID = 0
				}
			}
		}

		// Build helpful hint with concrete values
		var hint string
		if len(resp.Results) == 0 {
			hint = "No matches found. Check session_id is correct and filters aren't too restrictive."
		} else if resp.TotalHint > len(resp.Results) {
			nextOffset := input.Offset + len(resp.Results)
			hint = fmt.Sprintf("Showing %d of ~%d. Add host/path filters to narrow, or use offset=%d for next page.", len(resp.Results), resp.TotalHint, nextOffset)
		} else if len(resp.Results) == 1 && resp.Results[0].Summary != nil {
			hint = fmt.Sprintf("Single match. Use get_entry(entry_id=%q) for full details.", resp.Results[0].Summary.EntryID)
		} else {
			hint = "Use get_entry with an entry_id for details, or extract_endpoints to see API patterns."
		}

		return nil, SearchEntriesOutput{
			Results:       resp.Results,
			TotalHint:     resp.TotalHint,
			SyncedAtMs:    resp.SyncedAtMs,
			SearchedScope: resp.Scope,
			Hint:          hint,
		}, nil
	}
}
