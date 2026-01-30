package tools

import (
	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/catalog"
	"github.com/usestring/powhttp-mcp/internal/compare"
	"github.com/usestring/powhttp-mcp/internal/config"
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
