package client

import (
	"context"
	"fmt"
	"net/url"
)

// GetWebSocketMessages retrieves all WebSocket messages for an entry.
// Returns an error if the entry is not a WebSocket connection.
// Use "active" as sessionID or entryID to reference currently active resources.
func (c *Client) GetWebSocketMessages(ctx context.Context, sessionID, entryID string) ([]WebSocketMessage, error) {
	path := "/sessions/" + url.PathEscape(sessionID) + "/entries/" + url.PathEscape(entryID) + "/websocket"
	var messages []WebSocketMessage
	if err := c.get(ctx, path, nil, &messages); err != nil {
		return nil, fmt.Errorf("getting websocket messages for entry %q in session %q: %w", entryID, sessionID, err)
	}
	return messages, nil
}
