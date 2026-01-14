// Package client provides a Go SDK for the powhttp Data API.
//
// The powhttp Data API provides a RESTful HTTP interface to programmatically
// access session data, entries, and connection details from powhttp. This SDK
// enables building custom integrations, exporting data to external tools, or
// automating workflows based on captured network traffic.
//
// # Quick Start
//
// Create a client and list sessions:
//
//	c := client.New()
//	sessions, err := c.ListSessions(ctx)
//
// Use custom configuration:
//
//	c := client.New(
//	    client.WithBaseURL("http://localhost:8080"),
//	    client.WithHTTPClient(customHTTPClient),
//	)
//
// # The "active" Identifier
//
// Many methods accept "active" as a special identifier to reference the
// currently active session or entry in the powhttp interface:
//
//	// Get the currently active session
//	session, err := c.GetSession(ctx, "active")
//
//	// Get the currently active entry in the active session
//	entry, err := c.GetEntry(ctx, "active", "active")
//
// # Entry Filtering
//
// The ListEntries method supports filtering by selection state, bookmarks,
// and highlight colors:
//
//	entries, err := c.ListEntries(ctx, "active", &client.ListEntriesOptions{
//	    Selected:    true,
//	    Bookmarked:  true,
//	    Highlighted: []string{client.HighlightRed, client.HighlightYellow},
//	})
//
// # Working with Bodies
//
// Request and response bodies are base64-encoded. Use DecodeBody to decode them:
//
//	body, err := client.DecodeBody(entry.Response.Body)
//
// # Working with Headers
//
// Headers are returned as a slice of key-value pairs. The Headers type provides
// helper methods for common operations:
//
//	contentType := entry.Response.Headers.Get("Content-Type")
//	cookies := entry.Response.Headers.Values("Set-Cookie")
//
// # TLS and HTTP/2 Details
//
// For entries using TLS or HTTP/2, you can retrieve detailed connection
// information. The connection ID is found in the entry's TLS or HTTP2 fields:
//
//	// Get TLS handshake events
//	if entry.TLS.ConnectionID != nil {
//	    events, err := c.GetTLSConnection(ctx, *entry.TLS.ConnectionID)
//	}
//
//	// Get HTTP/2 stream details
//	if entry.HTTP2 != nil {
//	    frames, err := c.GetHTTP2Stream(ctx, entry.HTTP2.ConnectionID, entry.HTTP2.StreamID)
//	}
//
// TLS events and HTTP/2 frames are returned as json.RawMessage slices since
// their schemas vary by event/frame type.
package client
