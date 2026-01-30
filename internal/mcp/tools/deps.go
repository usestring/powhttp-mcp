package tools

import (
	"context"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/catalog"
	"github.com/usestring/powhttp-mcp/internal/compare"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/entryfetch"
	"github.com/usestring/powhttp-mcp/internal/flow"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/internal/search"
	"github.com/usestring/powhttp-mcp/pkg/client"
)

// Deps contains all dependencies needed by tool handlers.
type Deps struct {
	Client       *client.Client
	Indexer      *indexer.Indexer
	Cache        *cache.EntryCache
	Config       *config.Config
	Search       *search.SearchEngine
	Fingerprint  *compare.FingerprintEngine
	Diff         *compare.DiffEngine
	Cluster      *catalog.ClusterEngine
	Describe     *catalog.DescribeEngine
	ClusterStore *catalog.ClusterStore
	Flow         *flow.FlowEngine
}

// FetchEntry retrieves an entry by ID, checking the cache first.
// If not cached, it fetches from the API client and caches the result.
func (d *Deps) FetchEntry(ctx context.Context, sessionID, entryID string) (*client.SessionEntry, error) {
	return entryfetch.FetchEntry(ctx, d.Client, d.Cache, sessionID, entryID)
}

// DecodeBody extracts and base64-decodes the body for a given target
// ("request" or "response"). Returns the decoded bytes and the content-type
// header value. Does not filter by content type.
func (d *Deps) DecodeBody(entry *client.SessionEntry, target string) ([]byte, string, error) {
	return entryfetch.DecodeBody(entry, target)
}
