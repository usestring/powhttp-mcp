// Package types provides shared types for powhttp-mcp.
// These types are used across multiple packages and are designed for external consumption.
package types

// ToolError represents a structured error response for MCP tools.
type ToolError struct {
	Code    string `json:"code"` // NOT_FOUND, POWHTTP_ERROR, INVALID_INPUT, TIMEOUT
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
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
