// Package types provides shared types for powhttp-mcp.
// These types are used across multiple packages and are designed for external consumption.
package types

import "encoding/json"

// ToAny round-trips a typed value through JSON to produce an untyped any.
// Use this when a tool output field must be any (instead of json.RawMessage)
// to satisfy the MCP SDK's schema validation.
func ToAny(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EntrySummary is a compact entry representation for search results.
type EntrySummary struct {
	EntryID     string       `json:"entry_id"`
	TsMs        int64        `json:"ts_ms"`
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	Host        string       `json:"host"`
	Path        string       `json:"path"`
	Status      int          `json:"status"`
	HTTPVersion string       `json:"http_version"`
	ProcessName string       `json:"process_name"`
	PID         int          `json:"pid"`
	TLS         TLSSummary   `json:"tls"`
	HTTP2       HTTP2Summary `json:"http2,omitempty"`
	Sizes       SizeSummary  `json:"sizes,omitempty"`
}

// TLSSummary contains TLS-related summary fields.
type TLSSummary struct {
	ConnectionID string `json:"connection_id,omitempty"`
	JA3          string `json:"ja3,omitempty"`
	JA4          string `json:"ja4,omitempty"`
}

// HTTP2Summary contains HTTP/2-related summary fields.
type HTTP2Summary struct {
	ConnectionID string `json:"connection_id,omitempty"`
	StreamID     int    `json:"stream_id,omitempty"`
}

// SizeSummary contains request/response body size information.
type SizeSummary struct {
	ReqBodyBytes    int    `json:"req_body_bytes"`
	RespBodyBytes   int    `json:"resp_body_bytes"`
	RespContentType string `json:"resp_content_type,omitempty"` // e.g., "application/json"
}

// ResourceRef points to an MCP resource.
type ResourceRef struct {
	URI  string `json:"uri"`
	MIME string `json:"mime"`
	Hint string `json:"hint,omitempty"`
}
