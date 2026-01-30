package entryfetch

import (
	"context"
	"encoding/base64"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/pkg/client"
)

// FetchEntry retrieves an entry by ID, checking the cache first.
// If not cached, it fetches from the API client and caches the result.
func FetchEntry(ctx context.Context, c *client.Client, ec *cache.EntryCache, sessionID, entryID string) (*client.SessionEntry, error) {
	if cached, ok := ec.Get(entryID); ok {
		return cached, nil
	}

	entry, err := c.GetEntry(ctx, sessionID, entryID)
	if err != nil {
		return nil, err
	}

	ec.Put(entryID, entry)
	return entry, nil
}

// DecodeBody extracts and base64-decodes the body for a given target
// ("request" or "response"). Returns the decoded bytes and the content-type
// header value. Does not filter by content type.
//
// Returns (nil, "", nil) when the entry has no response (target "response")
// or the body is nil/empty.
func DecodeBody(entry *client.SessionEntry, target string) ([]byte, string, error) {
	var body *string
	var contentType string

	if target == "request" {
		body = entry.Request.Body
		contentType = entry.Request.Headers.Get("content-type")
	} else {
		if entry.Response == nil {
			return nil, "", nil
		}
		body = entry.Response.Body
		contentType = entry.Response.Headers.Get("content-type")
	}

	if body == nil || *body == "" {
		return nil, contentType, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(*body)
	if err != nil {
		return nil, contentType, err
	}

	return decoded, contentType, nil
}
