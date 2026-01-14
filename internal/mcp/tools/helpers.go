// Package tools contains MCP tool implementations for powhttp.
package tools

import (
	"encoding/json"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// MIME type constant.
const MimeJSON = "application/json"

// MakeJSONToolResult creates a CallToolResult with JSON text content.
func MakeJSONToolResult(v any) (*sdkmcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: string(b)},
		},
	}, nil
}

// IsErrLine checks if a stderr line indicates an error.
func IsErrLine(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "error") || strings.Contains(l, "exception") || strings.Contains(l, "fatal")
}

// BuildEntrySummaryFromEntry creates an EntrySummary from a SessionEntry.
func BuildEntrySummaryFromEntry(entry *client.SessionEntry) *types.EntrySummary {
	summary := &types.EntrySummary{
		EntryID:     entry.ID,
		TsMs:        entry.Timings.StartedAt,
		URL:         entry.URL,
		HTTPVersion: entry.HTTPVersion,
	}

	if entry.Request.Method != nil {
		summary.Method = *entry.Request.Method
	}
	if entry.Response != nil && entry.Response.StatusCode != nil {
		summary.Status = *entry.Response.StatusCode
	}
	if entry.Process != nil {
		summary.PID = entry.Process.PID
		if entry.Process.Name != nil {
			summary.ProcessName = *entry.Process.Name
		}
	}
	if entry.TLS.ConnectionID != nil {
		summary.TLS.ConnectionID = *entry.TLS.ConnectionID
	}
	if entry.TLS.JA3 != nil {
		summary.TLS.JA3 = entry.TLS.JA3.Hash
	}
	if entry.TLS.JA4 != nil {
		summary.TLS.JA4 = entry.TLS.JA4.Hashed
	}
	if entry.HTTP2 != nil {
		summary.HTTP2.ConnectionID = entry.HTTP2.ConnectionID
		summary.HTTP2.StreamID = entry.HTTP2.StreamID
	}

	return summary
}
