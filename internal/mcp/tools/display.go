package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

// DisplayEntry is a transformed version of SessionEntry with decoded bodies.
type DisplayEntry struct {
	ID              string                `json:"id"`
	URL             string                `json:"url"`
	ClientAddr      *client.SocketAddress `json:"clientAddr,omitempty"`
	RemoteAddr      *client.SocketAddress `json:"remoteAddr,omitempty"`
	HTTPVersion     string                `json:"httpVersion"`
	TransactionType string                `json:"transactionType"`
	Request         DisplayRequest        `json:"request"`
	Response        *DisplayResponse      `json:"response,omitempty"`
	IsWebSocket     bool                  `json:"isWebSocket"`
	TLS             client.TLSInfo        `json:"tls"`
	HTTP2           *client.HTTP2Info     `json:"http2,omitempty"`
	Timings         client.Timings        `json:"timings"`
	Process         *client.ProcessInfo   `json:"process,omitempty"`
}

// DisplayRequest is a transformed Request with decoded body.
type DisplayRequest struct {
	Method      *string        `json:"method,omitempty"`
	Path        *string        `json:"path,omitempty"`
	HTTPVersion *string        `json:"httpVersion,omitempty"`
	Headers     client.Headers `json:"headers"`
	Body        string         `json:"body,omitempty"` // text content, JSON schema, or placeholder
}

// DisplayResponse is a transformed Response with decoded body.
type DisplayResponse struct {
	HTTPVersion *string        `json:"httpVersion,omitempty"`
	StatusCode  *int           `json:"statusCode,omitempty"`
	StatusText  *string        `json:"statusText,omitempty"`
	Headers     client.Headers `json:"headers"`
	Body        string         `json:"body,omitempty"` // text content, JSON schema, or placeholder
}

// BodyTransformOptions controls how bodies are transformed for display.
type BodyTransformOptions struct {
	MaxBytes       int
	SchemaOnly     bool
	IncludeHeaders bool // If false, headers are omitted from output
}

// TransformBody decodes a base64 body for display.
// Returns the transformed body content as a string (text content, JSON schema, or placeholder).
func TransformBody(encoded *string, contentType string, opts BodyTransformOptions) string {
	if encoded == nil || *encoded == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		return fmt.Sprintf("[decode error: %s]", err.Error())
	}

	totalBytes := len(decoded)

	// Check if binary
	if isBinaryContent(contentType, decoded) {
		return fmt.Sprintf("[binary content, %d bytes]", totalBytes)
	}

	// If schema mode and JSON content type, return schema as JSON string
	if opts.SchemaOnly && isJSONContentType(contentType) {
		inferred, err := jsonschema.Infer(decoded)
		if err == nil && inferred != nil {
			schemaJSON, err := json.Marshal(inferred.Schema)
			if err == nil {
				return string(schemaJSON)
			}
		}
		// Fall through to text display if schema inference fails
	}

	// Return as text, potentially truncated
	text := string(decoded)
	if opts.MaxBytes > 0 && totalBytes > opts.MaxBytes {
		// Truncate at valid UTF-8 boundary
		truncated := truncateUTF8(text, opts.MaxBytes)
		return fmt.Sprintf("%s\n... [truncated, showing %d of %d bytes]", truncated, len(truncated), totalBytes)
	}

	return text
}

// isBinaryContent determines if content should be treated as binary.
func isBinaryContent(contentType string, data []byte) bool {
	ct := strings.ToLower(contentType)

	// Text content types
	if strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "javascript") ||
		strings.Contains(ct, "html") ||
		strings.Contains(ct, "css") ||
		strings.Contains(ct, "yaml") ||
		strings.Contains(ct, "form-urlencoded") {
		return false
	}

	// Explicit binary types
	if strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "audio/") ||
		strings.HasPrefix(ct, "video/") ||
		strings.Contains(ct, "octet-stream") ||
		strings.Contains(ct, "gzip") ||
		strings.Contains(ct, "zip") ||
		strings.Contains(ct, "pdf") {
		return true
	}

	// Unknown content type - check if valid UTF-8
	if contentType == "" {
		return !utf8.Valid(data)
	}

	// Default to text for unknown types if UTF-8 valid
	return !utf8.Valid(data)
}

// isJSONContentType checks if content type indicates JSON.
func isJSONContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json")
}

// truncateUTF8 truncates a string at a valid UTF-8 boundary.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// Find valid UTF-8 boundary
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}

	return s[:maxBytes]
}

// ToDisplayEntry converts a SessionEntry to DisplayEntry with decoded bodies.
func ToDisplayEntry(entry *client.SessionEntry, opts BodyTransformOptions) *DisplayEntry {
	reqContentType := entry.Request.Headers.Get("content-type")
	var respContentType string
	if entry.Response != nil {
		respContentType = entry.Response.Headers.Get("content-type")
	}

	var reqHeaders client.Headers
	if opts.IncludeHeaders {
		reqHeaders = entry.Request.Headers
	} else {
		reqHeaders = client.Headers{}
	}

	display := &DisplayEntry{
		ID:              entry.ID,
		URL:             entry.URL,
		ClientAddr:      entry.ClientAddr,
		RemoteAddr:      entry.RemoteAddr,
		HTTPVersion:     entry.HTTPVersion,
		TransactionType: entry.TransactionType,
		Request: DisplayRequest{
			Method:      entry.Request.Method,
			Path:        entry.Request.Path,
			HTTPVersion: entry.Request.HTTPVersion,
			Headers:     reqHeaders,
			Body:        TransformBody(entry.Request.Body, reqContentType, opts),
		},
		IsWebSocket: entry.IsWebSocket,
		TLS:         entry.TLS,
		HTTP2:       entry.HTTP2,
		Timings:     entry.Timings,
		Process:     entry.Process,
	}

	if entry.Response != nil {
		var respHeaders client.Headers
		if opts.IncludeHeaders {
			respHeaders = entry.Response.Headers
		} else {
			respHeaders = client.Headers{}
		}

		display.Response = &DisplayResponse{
			HTTPVersion: entry.Response.HTTPVersion,
			StatusCode:  entry.Response.StatusCode,
			StatusText:  entry.Response.StatusText,
			Headers:     respHeaders,
			Body:        TransformBody(entry.Response.Body, respContentType, opts),
		}
	}

	return display
}
