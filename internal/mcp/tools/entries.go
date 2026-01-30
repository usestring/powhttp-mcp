package tools

import (
	"context"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/jsoncompact"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// GetEntryInput is the input for powhttp_get_entry.
type GetEntryInput struct {
	SessionID      string `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryID        string `json:"entry_id" jsonschema:"required,Entry ID to retrieve"`
	MaxBytes       int    `json:"max_bytes,omitempty" jsonschema:"Max body bytes to return (for full mode)"`
	BodyMode       string `json:"body_mode,omitempty" jsonschema:"Body display mode: compact (default - arrays trimmed), schema (JSON schema only), full (complete body)"`
	IncludeHeaders bool   `json:"include_headers,omitempty" jsonschema:"Include request/response headers (default: false)"`
}

// GetEntryOutput is the output for powhttp_get_entry.
type GetEntryOutput struct {
	Summary       *types.EntrySummary `json:"summary"`
	Entry         *DisplayEntry       `json:"entry,omitempty"`
	AvailableData *AvailableData      `json:"available_data,omitempty"`
	Resource      *types.ResourceRef  `json:"resource,omitempty"`
	Truncated     bool                `json:"truncated,omitempty"`
	Truncation    *Truncation         `json:"truncation,omitempty"`
	Hint          string              `json:"hint,omitempty"`
}

// AvailableData reports what data was included in the response.
type AvailableData struct {
	HeadersIncluded bool   `json:"headers_included"`
	BodyMode        string `json:"body_mode"`
	RespBodyBytes   int    `json:"resp_body_bytes,omitempty"`
	RespContentType string `json:"resp_content_type,omitempty"`
	ReqBodyBytes    int    `json:"req_body_bytes,omitempty"`
}

// Truncation describes what was truncated.
type Truncation struct {
	ReqBodyTruncated  bool `json:"req_body_truncated,omitempty"`
	RespBodyTruncated bool `json:"resp_body_truncated,omitempty"`
}

// GetTLSInput is the input for powhttp_get_tls.
type GetTLSInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,TLS connection ID"`
	MaxEvents    int    `json:"max_events,omitempty" jsonschema:"Max events to return"`
}

// GetTLSOutput is the output for powhttp_get_tls.
type GetTLSOutput struct {
	Summary   *TLSConnectionSummary `json:"summary"`
	Resource  *types.ResourceRef    `json:"resource,omitempty"`
	Truncated bool                  `json:"truncated,omitempty"`
}

// TLSConnectionSummary summarizes a TLS connection.
type TLSConnectionSummary struct {
	ConnectionID string `json:"connection_id"`
	EventCount   int    `json:"event_count"`
	TLSVersion   string `json:"tls_version,omitempty"`
	CipherSuite  string `json:"cipher_suite,omitempty"`
}

// GetHTTP2StreamInput is the input for powhttp_get_http2_stream.
type GetHTTP2StreamInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"required,HTTP/2 connection ID"`
	StreamID     int    `json:"stream_id" jsonschema:"required,Stream ID"`
	MaxEvents    int    `json:"max_events,omitempty" jsonschema:"Max frames to return"`
}

// GetHTTP2StreamOutput is the output for powhttp_get_http2_stream.
type GetHTTP2StreamOutput struct {
	Summary   *HTTP2StreamSummary `json:"summary"`
	Resource  *types.ResourceRef  `json:"resource,omitempty"`
	Truncated bool                `json:"truncated,omitempty"`
}

// HTTP2StreamSummary summarizes an HTTP/2 stream.
type HTTP2StreamSummary struct {
	ConnectionID string         `json:"connection_id"`
	StreamID     int            `json:"stream_id"`
	FrameCount   int            `json:"frame_count"`
	FrameTypes   map[string]int `json:"frame_types,omitempty"`
}

// ToolGetEntry gets a specific entry.
func ToolGetEntry(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetEntryInput) (*sdkmcp.CallToolResult, GetEntryOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetEntryInput) (*sdkmcp.CallToolResult, GetEntryOutput, error) {
		if input.EntryID == "" {
			return nil, GetEntryOutput{}, ErrInvalidInput("entry_id is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		// Validate body mode
		bodyMode := input.BodyMode
		if bodyMode == "" {
			bodyMode = "compact"
		}
		if bodyMode != "compact" && bodyMode != "schema" && bodyMode != "full" {
			return nil, GetEntryOutput{}, ErrInvalidInput("body_mode must be 'compact', 'schema', or 'full'")
		}

		// Try cache first
		var entry *client.SessionEntry
		if cached, ok := d.Cache.Get(input.EntryID); ok {
			entry = cached
		} else {
			var err error
			entry, err = d.Client.GetEntry(ctx, sessionID, input.EntryID)
			if err != nil {
				return nil, GetEntryOutput{}, WrapPowHTTPError(err)
			}
			d.Cache.Put(input.EntryID, entry)
		}

		// Build summary from metadata if available
		meta := d.Indexer.GetMetaByEntryID(input.EntryID)
		var summary *types.EntrySummary
		if meta != nil {
			summary = meta.ToSummary()
		} else {
			summary = BuildEntrySummaryFromEntry(entry)
		}

		// Build transform options based on body mode
		opts := BodyTransformOptions{
			IncludeHeaders: input.IncludeHeaders,
		}
		switch bodyMode {
		case "compact":
			opts.CompactArrays = true
			opts.CompactOptions = &jsoncompact.Options{
				MaxArrayItems: d.Config.CompactMaxArrayItems,
				MaxStringLen:  d.Config.CompactMaxStringLen,
				MaxDepth:      d.Config.CompactMaxDepth,
			}
		case "schema":
			opts.SchemaOnly = true
		case "full":
			if input.MaxBytes > 0 {
				opts.MaxBytes = input.MaxBytes
			}
			// No limit for full mode unless explicitly set
		}

		// Transform entry for display
		displayEntry := ToDisplayEntry(entry, opts)

		// Build resource URI - resource always has full body
		resourceURI := "powhttp://entry/" + sessionID + "/" + input.EntryID

		// Check if response is JSON for contextual hints
		var respContentType string
		if entry.Response != nil {
			respContentType = entry.Response.Headers.Get("content-type")
		}
		isJSONResp := isJSONContentType(respContentType)

		// Customize hint based on body mode and content type
		var hint string
		switch bodyMode {
		case "compact":
			if isJSONResp {
				hint = "Arrays trimmed. Use query_body to extract specific fields, or body_mode='full' for complete data."
			} else {
				hint = "Use trace_flow to find related requests."
			}
		case "schema":
			hint = "Schema-only view. Use body_mode='full' for actual values or query_body for specific fields."
		case "full":
			if isJSONResp {
				hint = "Use query_body with expression to extract specific values."
			} else {
				hint = "Entry complete. Use trace_flow to find related requests."
			}
		}

		// Build available data hint
		availableData := &AvailableData{
			HeadersIncluded: input.IncludeHeaders,
			BodyMode:        bodyMode,
		}
		if summary != nil {
			availableData.RespBodyBytes = summary.Sizes.RespBodyBytes
			availableData.RespContentType = summary.Sizes.RespContentType
			availableData.ReqBodyBytes = summary.Sizes.ReqBodyBytes
		}

		output := GetEntryOutput{
			Summary:       summary,
			Entry:         displayEntry,
			AvailableData: availableData,
			Resource: &types.ResourceRef{
				URI:  resourceURI,
				MIME: MimeJSON,
				Hint: "Fetch for complete request/response with full bodies.",
			},
			Hint: hint,
		}

		return nil, output, nil
	}
}

// ToolGetTLS gets TLS connection details.
func ToolGetTLS(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetTLSInput) (*sdkmcp.CallToolResult, GetTLSOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetTLSInput) (*sdkmcp.CallToolResult, GetTLSOutput, error) {
		if input.ConnectionID == "" {
			return nil, GetTLSOutput{}, ErrInvalidInput("connection_id is required")
		}

		events, err := d.Client.GetTLSConnection(ctx, input.ConnectionID)
		if err != nil {
			return nil, GetTLSOutput{}, WrapPowHTTPError(err)
		}

		maxEvents := input.MaxEvents
		if maxEvents <= 0 {
			maxEvents = d.Config.TLSMaxEventsDefault
		}

		truncated := len(events) > maxEvents
		if truncated {
			events = events[:maxEvents]
		}

		// Extract TLS version and cipher from events
		summary := &TLSConnectionSummary{
			ConnectionID: input.ConnectionID,
			EventCount:   len(events),
		}
		for _, event := range events {
			if event.Msg.Type == client.TLSMsgHandshake && event.Msg.Handshake != nil {
				if event.Msg.Handshake.Type == client.TLSHandshakeServerHello && event.Msg.Handshake.ServerHello != nil {
					summary.TLSVersion = event.Msg.Handshake.ServerHello.Version.Name
					summary.CipherSuite = event.Msg.Handshake.ServerHello.CipherSuite.Name
					break
				}
			}
		}

		return nil, GetTLSOutput{
			Summary: summary,
			Resource: &types.ResourceRef{
				URI:  "powhttp://tls/" + input.ConnectionID,
				MIME: MimeJSON,
				Hint: "Fetch for raw TLS handshake frame details",
			},
			Truncated: truncated,
		}, nil
	}
}

// ToolGetHTTP2Stream gets HTTP/2 stream details.
func ToolGetHTTP2Stream(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetHTTP2StreamInput) (*sdkmcp.CallToolResult, GetHTTP2StreamOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input GetHTTP2StreamInput) (*sdkmcp.CallToolResult, GetHTTP2StreamOutput, error) {
		if input.ConnectionID == "" {
			return nil, GetHTTP2StreamOutput{}, ErrInvalidInput("connection_id is required")
		}

		frames, err := d.Client.GetHTTP2Stream(ctx, input.ConnectionID, input.StreamID)
		if err != nil {
			return nil, GetHTTP2StreamOutput{}, WrapPowHTTPError(err)
		}

		maxEvents := input.MaxEvents
		if maxEvents <= 0 {
			maxEvents = d.Config.H2MaxEventsDefault
		}

		truncated := len(frames) > maxEvents
		if truncated {
			frames = frames[:maxEvents]
		}

		// Count frame types
		frameTypes := make(map[string]int)
		for _, raw := range frames {
			var frame struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &frame) == nil && frame.Type != "" {
				frameTypes[frame.Type]++
			}
		}

		return nil, GetHTTP2StreamOutput{
			Summary: &HTTP2StreamSummary{
				ConnectionID: input.ConnectionID,
				StreamID:     input.StreamID,
				FrameCount:   len(frames),
				FrameTypes:   frameTypes,
			},
			Resource: &types.ResourceRef{
				URI:  "powhttp://http2/" + input.ConnectionID + "/" + string(rune(input.StreamID)),
				MIME: MimeJSON,
				Hint: "Fetch for raw HTTP/2 frame-level data",
			},
			Truncated: truncated,
		}, nil
	}
}
