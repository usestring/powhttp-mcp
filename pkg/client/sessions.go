package client

import (
	"context"
	"fmt"
	"net/url"
)

// ListSessions retrieves all sessions currently loaded in powhttp.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.get(ctx, "/sessions", nil, &sessions); err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
}

// GetSession retrieves a specific session by ID.
// Use "active" as the sessionID to get the currently active session.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	path := "/sessions/" + url.PathEscape(sessionID)
	var session Session
	if err := c.get(ctx, path, nil, &session); err != nil {
		return nil, fmt.Errorf("getting session %q: %w", sessionID, err)
	}
	return &session, nil
}

// GetSessionBookmarks retrieves bookmarked entry IDs for a session.
// Use "active" as the sessionID to reference the currently active session.
func (c *Client) GetSessionBookmarks(ctx context.Context, sessionID string) ([]string, error) {
	path := "/sessions/" + url.PathEscape(sessionID) + "/bookmarks"
	var bookmarks []string
	if err := c.get(ctx, path, nil, &bookmarks); err != nil {
		return nil, fmt.Errorf("getting bookmarks for session %q: %w", sessionID, err)
	}
	return bookmarks, nil
}
