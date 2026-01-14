package tools

import (
	"context"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// GetEntryInput is the input for powhttp_get_entry.
type GetEntryInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryID   string `json:"entry_id" jsonschema:"required,Entry ID to retrieve"`
	MaxBytes  int    `json:"max_bytes,omitempty" jsonschema:"Max body bytes to return"`
}

// GetEntryOutput is the output for powhttp_get_entry.
type GetEntryOutput struct {
	Summary    *types.EntrySummary `json:"summary"`
	Entry      *DisplayEntry       `json:"entry,omitempty"`
	Resource   *types.ResourceRef  `json:"resource,omitempty"`
	Truncated  bool                `json:"truncated,omitempty"`
	Truncation *Truncation         `json:"truncation,omitempty"`
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

		// Transform entry for display with schema (resource has full body)
		displayEntry := ToDisplayEntry(entry, BodyTransformOptions{SchemaOnly: true})

		// Build resource URI - resource always has full body
		resourceURI := "powhttp://entry/" + sessionID + "/" + input.EntryID

		output := GetEntryOutput{
			Summary: summary,
			Entry:   displayEntry,
			Resource: &types.ResourceRef{
				URI:  resourceURI,
				MIME: MimeJSON,
				Hint: "Fetch this resource for full request/response bodies",
			},
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
