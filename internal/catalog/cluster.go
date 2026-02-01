package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RoaringBitmap/roaring/v2"

	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ClusterEngine builds endpoint clusters from indexed entries.
type ClusterEngine struct {
	indexer *indexer.Indexer
	config  *config.Config
	store   *ClusterStore
}

// NewClusterEngine creates a new ClusterEngine.
func NewClusterEngine(idx *indexer.Indexer, cfg *config.Config, store *ClusterStore) *ClusterEngine {
	return &ClusterEngine{
		indexer: idx,
		config:  cfg,
		store:   store,
	}
}

// Extract builds endpoint clusters from indexed entries.
func (c *ClusterEngine) Extract(ctx context.Context, req *types.ExtractRequest) (*types.ExtractResponse, error) {
	// Ensure index is fresh
	if err := c.indexer.RefreshIfStale(ctx, req.SessionID); err != nil {
		return nil, fmt.Errorf("refreshing index: %w", err)
	}

	// Apply defaults
	opts := applyOptionsDefaults(req.Options)
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	// Get candidate doc IDs based on scope
	candidates := c.applyScopeFilters(req.Scope)

	// Build clusters from candidates
	clusterMap := make(map[types.ClusterKey]*clusterBuilder)
	fullEntryIDs := make(map[string][]string)

	iter := candidates.Iterator()
	for iter.HasNext() {
		docID := iter.Next()
		meta := c.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}

		// Apply time filters
		if req.Scope != nil {
			if !c.matchesTimeScope(meta, req.Scope) {
				continue
			}
		}

		// Build path template
		pathTemplate := buildPathTemplate(meta.Path, opts.NormalizeIDs)

		key := types.ClusterKey{
			Host:         meta.Host,
			Method:       meta.Method,
			PathTemplate: pathTemplate,
		}

		builder, exists := clusterMap[key]
		if !exists {
			builder = &clusterBuilder{
				key:          key,
				entryIDs:     make([]string, 0),
				contentTypes: make(map[string]int),
				statusCounts: make(map[int]int),
			}
			clusterMap[key] = builder
		}
		builder.entryIDs = append(builder.entryIDs, meta.EntryID)
		if meta.RespContentType != "" {
			builder.contentTypes[meta.RespContentType]++
		}
		if meta.Status != 0 {
			builder.statusCounts[meta.Status]++
		}
		builder.totalRespBytes += int64(meta.RespBodyBytes)
		if !builder.hasAuth {
			if meta.AuthHeader != "" || len(meta.Cookies) > 0 || len(meta.APIKeys) > 0 {
				builder.hasAuth = true
			}
		}
	}

	// Convert to clusters, respecting MaxClusters
	builders := make([]*clusterBuilder, 0, len(clusterMap))
	for _, b := range clusterMap {
		b.category = classifyCluster(b.key, b.contentTypes)
		builders = append(builders, b)
	}

	// Apply post-clustering filters (category, min_count)
	builders = applyPostClusterFilters(builders, req.Filters)

	// Sort by count descending (most frequent endpoints first)
	sort.Slice(builders, func(i, j int) bool {
		return len(builders[i].entryIDs) > len(builders[j].entryIDs)
	})

	// Apply MaxClusters limit
	maxClusters := opts.MaxClusters
	if len(builders) > maxClusters {
		builders = builders[:maxClusters]
	}

	totalCount := len(builders)

	// Apply pagination
	start := req.Offset
	if start > len(builders) {
		start = len(builders)
	}
	end := start + limit
	if end > len(builders) {
		end = len(builders)
	}
	paginatedBuilders := builders[start:end]

	// Build final clusters
	clusters := make([]types.Cluster, 0, len(paginatedBuilders))
	for _, b := range paginatedBuilders {
		clusterID := computeClusterID(b.key)
		examples := selectExamples(b.entryIDs, opts.ExamplesPerCluster)

		cluster := types.Cluster{
			ID:              clusterID,
			Host:            b.key.Host,
			Method:          b.key.Method,
			PathTemplate:    b.key.PathTemplate,
			Count:           len(b.entryIDs),
			Category:        b.category,
			Stats:           computeClusterStats(b),
			ExampleEntryIDs: examples,
		}

		// Compute content type hint: most common response content type
		if len(b.contentTypes) > 0 {
			cluster.ContentTypeHint = dominantContentType(b.contentTypes)
		}

		clusters = append(clusters, cluster)

		fullEntryIDs[clusterID] = b.entryIDs
	}

	scopeHash := computeScopeHash(req.Scope, req.Filters)

	resp := &types.ExtractResponse{
		Clusters:   clusters,
		TotalCount: totalCount,
		ScopeHash:  scopeHash,
	}

	// Store the extraction for later use
	if c.store != nil {
		c.store.StoreExtraction(resp, fullEntryIDs)
	}

	return resp, nil
}

// clusterBuilder accumulates entries for a single cluster during extraction.
type clusterBuilder struct {
	key            types.ClusterKey
	entryIDs       []string
	contentTypes   map[string]int
	statusCounts   map[int]int // exact status code -> count
	totalRespBytes int64       // sum of all response body sizes
	hasAuth        bool        // any entry had auth signals
	category       types.EndpointCategory
}

// applyOptionsDefaults applies default values to ClusterOptions.
func applyOptionsDefaults(opts *types.ClusterOptions) *types.ClusterOptions {
	result := &types.ClusterOptions{
		NormalizeIDs:           true,
		StripVolatileQueryKeys: true,
		ExamplesPerCluster:     3,
		MaxClusters:            200,
	}

	if opts == nil {
		return result
	}

	// Copy user-provided values
	result.NormalizeIDs = opts.NormalizeIDs
	result.StripVolatileQueryKeys = opts.StripVolatileQueryKeys

	if opts.ExamplesPerCluster > 0 {
		result.ExamplesPerCluster = opts.ExamplesPerCluster
		if result.ExamplesPerCluster > 10 {
			result.ExamplesPerCluster = 10
		}
	}

	if opts.MaxClusters > 0 {
		result.MaxClusters = opts.MaxClusters
		if result.MaxClusters > 2000 {
			result.MaxClusters = 2000
		}
	}

	return result
}

// applyScopeFilters returns a bitmap of doc IDs matching the scope.
func (c *ClusterEngine) applyScopeFilters(scope *types.ClusterScope) *roaring.Bitmap {
	result := c.indexer.AllDocIDs()

	if scope == nil {
		return result
	}

	if scope.Host != "" {
		if bm := c.indexer.GetBitmapForHost(strings.ToLower(scope.Host)); bm != nil {
			result = roaring.And(result, bm)
		} else {
			return roaring.New()
		}
	}

	if scope.Method != "" {
		if bm := c.indexer.GetBitmapForMethod(strings.ToUpper(scope.Method)); bm != nil {
			result = roaring.And(result, bm)
		} else {
			return roaring.New()
		}
	}

	if scope.ProcessName != "" {
		if bm := c.indexer.GetBitmapForProcessName(scope.ProcessName); bm != nil {
			result = roaring.And(result, bm)
		} else {
			return roaring.New()
		}
	}

	if scope.PID != 0 {
		if bm := c.indexer.GetBitmapForPID(scope.PID); bm != nil {
			result = roaring.And(result, bm)
		} else {
			return roaring.New()
		}
	}

	return result
}

// matchesTimeScope checks if an entry matches the time scope.
func (c *ClusterEngine) matchesTimeScope(meta *indexer.EntryMeta, scope *types.ClusterScope) bool {
	var sinceMs, untilMs int64

	if scope.TimeWindowMs > 0 {
		now := time.Now().UnixMilli()
		sinceMs = now - scope.TimeWindowMs
		untilMs = now
	} else {
		sinceMs = scope.SinceMs
		untilMs = scope.UntilMs
	}

	if sinceMs > 0 && meta.TsMs < sinceMs {
		return false
	}
	if untilMs > 0 && meta.TsMs > untilMs {
		return false
	}

	return true
}

// buildPathTemplate normalizes a path to a template.
// e.g., /users/123/posts -> /users/{id}/posts
func buildPathTemplate(path string, normalizeIDs bool) string {
	// Strip query string for template
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	if !normalizeIDs {
		return path
	}

	return indexer.NormalizePath(path)
}

// selectExamples picks representative entry IDs for a cluster.
// Selects entries spread across the list to get variety.
func selectExamples(entryIDs []string, count int) []string {
	if len(entryIDs) <= count {
		result := make([]string, len(entryIDs))
		copy(result, entryIDs)
		return result
	}

	result := make([]string, count)
	step := len(entryIDs) / count

	for i := 0; i < count; i++ {
		result[i] = entryIDs[i*step]
	}

	return result
}

// computeClusterID generates a deterministic cluster ID from the key.
func computeClusterID(key types.ClusterKey) string {
	data := key.Host + "\x00" + key.Method + "\x00" + key.PathTemplate
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:12]
}

// applyPostClusterFilters removes clusters that don't match post-clustering filters.
func applyPostClusterFilters(builders []*clusterBuilder, filters *types.ClusterFilters) []*clusterBuilder {
	if filters == nil {
		return builders
	}
	if filters.MinCount <= 0 && filters.Category == "" {
		return builders
	}

	filtered := make([]*clusterBuilder, 0, len(builders))
	for _, b := range builders {
		if filters.MinCount > 0 && len(b.entryIDs) < filters.MinCount {
			continue
		}
		if filters.Category != "" && b.category != filters.Category {
			continue
		}
		filtered = append(filtered, b)
	}
	return filtered
}

// computeClusterStats builds lightweight stats from accumulated builder data.
func computeClusterStats(b *clusterBuilder) types.ClusterStats {
	stats := types.ClusterStats{
		StatusProfile: make(map[string]int),
		HasAuth:       b.hasAuth,
	}

	var nonSuccess int
	total := len(b.entryIDs)
	for code, count := range b.statusCounts {
		bucket := statusBucket(code)
		stats.StatusProfile[bucket] += count
		if code < 200 || code >= 300 {
			nonSuccess += count
		}
	}

	if total > 0 {
		stats.ErrorRate = float64(nonSuccess) / float64(total)
		stats.AvgRespBytes = int(b.totalRespBytes / int64(total))
	}

	return stats
}

// statusBucket maps an HTTP status code to its class bucket.
func statusBucket(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "1xx"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "other"
	}
}

// computeScopeHash creates a hash for the scope and filters (for caching/resource URI).
func computeScopeHash(scope *types.ClusterScope, filters *types.ClusterFilters) string {
	if scope == nil && filters == nil {
		return "default"
	}

	var host, processName, method string
	var pid int
	var timeWindowMs, sinceMs, untilMs int64
	if scope != nil {
		host = scope.Host
		processName = scope.ProcessName
		pid = scope.PID
		timeWindowMs = scope.TimeWindowMs
		sinceMs = scope.SinceMs
		untilMs = scope.UntilMs
		method = scope.Method
	}

	var category string
	var minCount int
	if filters != nil {
		category = string(filters.Category)
		minCount = filters.MinCount
	}

	data := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d\x00%d\x00%s\x00%s\x00%d",
		host,
		processName,
		pid,
		timeWindowMs,
		sinceMs,
		untilMs,
		method,
		category,
		minCount,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}
