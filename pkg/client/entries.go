package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ListEntriesOptions contains optional filters for listing entries.
type ListEntriesOptions struct {
	// Selected filters to only return entries that are currently selected in the powhttp interface.
	Selected bool
	// Bookmarked filters to only return entries that are currently bookmarked.
	Bookmarked bool
	// Highlighted filters entries by their highlight state.
	// Use constants like HighlightRed, HighlightGreen, HighlightStrikethrough, etc.
	Highlighted []string
}

// ListEntries retrieves all entries within a session.
// Use "active" as the sessionID to reference the currently active session.
func (c *Client) ListEntries(ctx context.Context, sessionID string, opts *ListEntriesOptions) ([]SessionEntry, error) {
	path := "/sessions/" + url.PathEscape(sessionID) + "/entries"

	var query url.Values
	if opts != nil {
		query = make(url.Values)
		if opts.Selected {
			query.Set("selected", "")
		}
		if opts.Bookmarked {
			query.Set("bookmarked", "")
		}
		if len(opts.Highlighted) > 0 {
			query.Set("highlighted", strings.Join(opts.Highlighted, ","))
		}
	}

	var entries []SessionEntry
	if err := c.get(ctx, path, query, &entries); err != nil {
		return nil, fmt.Errorf("listing entries for session %q: %w", sessionID, err)
	}
	return entries, nil
}

// GetEntry retrieves a specific entry within a session.
// Use "active" as sessionID or entryID to reference currently active resources.
func (c *Client) GetEntry(ctx context.Context, sessionID, entryID string) (*SessionEntry, error) {
	path := "/sessions/" + url.PathEscape(sessionID) + "/entries/" + url.PathEscape(entryID)
	var entry SessionEntry
	if err := c.get(ctx, path, nil, &entry); err != nil {
		return nil, fmt.Errorf("getting entry %q in session %q: %w", entryID, sessionID, err)
	}
	return &entry, nil
}
