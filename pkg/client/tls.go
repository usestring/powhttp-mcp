package client

import (
	"context"
	"fmt"
	"net/url"
)

// GetTLSConnection retrieves TLS handshake events for a connection.
// The connectionID can be found in the TLS.ConnectionID field of an entry.
// Returns a slice of TLSEvent containing the handshake messages.
func (c *Client) GetTLSConnection(ctx context.Context, connectionID string) ([]TLSEvent, error) {
	path := "/tls/" + url.PathEscape(connectionID)
	var events []TLSEvent
	if err := c.get(ctx, path, nil, &events); err != nil {
		return nil, fmt.Errorf("getting TLS connection %q: %w", connectionID, err)
	}
	return events, nil
}
