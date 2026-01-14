package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ListHTTP2StreamIDs retrieves all stream IDs for an HTTP/2 connection.
// The connectionID can be found in the HTTP2.ConnectionID field of an entry.
func (c *Client) ListHTTP2StreamIDs(ctx context.Context, connectionID string) ([]int, error) {
	path := "/http2/" + url.PathEscape(connectionID)
	var streamIDs []int
	if err := c.get(ctx, path, nil, &streamIDs); err != nil {
		return nil, fmt.Errorf("listing HTTP/2 stream IDs for connection %q: %w", connectionID, err)
	}
	return streamIDs, nil
}

// GetHTTP2Stream retrieves frame-level details for a specific HTTP/2 stream.
// Returns raw JSON events since the schema varies by frame type.
func (c *Client) GetHTTP2Stream(ctx context.Context, connectionID string, streamID int) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/http2/%s/streams/%d", url.PathEscape(connectionID), streamID)
	var frames []json.RawMessage
	if err := c.get(ctx, path, nil, &frames); err != nil {
		return nil, fmt.Errorf("getting HTTP/2 stream %d for connection %q: %w", streamID, connectionID, err)
	}
	return frames, nil
}
