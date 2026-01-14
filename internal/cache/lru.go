// Package cache provides caching utilities for the MCP server.
package cache

import (
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

// EntryCache provides thread-safe LRU caching for full SessionEntry objects.
type EntryCache struct {
	cache *lru.Cache[string, *client.SessionEntry]
}

// NewEntryCache creates a new LRU cache with the specified maximum number of items.
func NewEntryCache(maxItems int) (*EntryCache, error) {
	c, err := lru.New[string, *client.SessionEntry](maxItems)
	if err != nil {
		return nil, err
	}
	return &EntryCache{cache: c}, nil
}

// Get retrieves an entry from the cache by its ID.
// Returns the entry and true if found, nil and false otherwise.
func (c *EntryCache) Get(entryID string) (*client.SessionEntry, bool) {
	return c.cache.Get(entryID)
}

// Put adds or updates an entry in the cache.
func (c *EntryCache) Put(entryID string, entry *client.SessionEntry) {
	c.cache.Add(entryID, entry)
}

// Len returns the current number of items in the cache.
func (c *EntryCache) Len() int {
	return c.cache.Len()
}
