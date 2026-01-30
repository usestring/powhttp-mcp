// Package search provides search capabilities over indexed HTTP entries.
package search

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RoaringBitmap/roaring/v2"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// SearchEngine provides search capabilities over the indexer.
type SearchEngine struct {
	indexer *indexer.Indexer
	cache   *cache.EntryCache
}

// New creates a new SearchEngine.
func New(idx *indexer.Indexer, c *cache.EntryCache) *SearchEngine {
	return &SearchEngine{indexer: idx, cache: c}
}

// Search executes a search with auto-refresh if stale.
func (s *SearchEngine) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	// Ensure index is fresh
	if err := s.indexer.RefreshIfStale(ctx, req.SessionID); err != nil {
		return nil, err
	}

	// Apply defaults
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Plan filters to get candidate bitmap
	candidates := s.planFilters(req.Filters, req.Query)

	// Apply time filters and post-filters (requires scanning metadata)
	postFilterResult := s.applyPostFilters(candidates, req.Filters)
	candidates = postFilterResult.bitmap

	// Get total count hint before pagination
	totalHint := int(candidates.GetCardinality())

	// Convert to slice and apply scoring
	docIDs := candidates.ToArray()
	results := s.scoreResults(docIDs, req)

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply pagination
	start := req.Offset
	if start > len(results) {
		start = len(results)
	}
	end := start + limit
	if end > len(results) {
		end = len(results)
	}
	paginated := results[start:end]

	// Build search scope
	var scope *types.SearchScope
	needsScope := req.Query != "" || (req.Filters != nil && (req.Filters.BodyContains != "" || req.Filters.HeaderContains != ""))
	if needsScope {
		scope = &types.SearchScope{
			BodyIndexEnabled: s.indexer.BodyIndexEnabled(),
		}
		if req.Filters != nil && req.Filters.BodyContains != "" {
			total := postFilterResult.bodyCacheHits + postFilterResult.bodyCacheMisses
			if total > 0 {
				scope.BodySearchCoverage = fmt.Sprintf("partial (%d/%d entries cached)", postFilterResult.bodyCacheHits, total)
				if postFilterResult.bodyCacheMisses == 0 {
					scope.BodySearchCoverage = fmt.Sprintf("full (%d entries searched)", total)
				}
			}
		}
	}

	return &types.SearchResponse{
		Results:    paginated,
		TotalHint:  totalHint,
		SyncedAtMs: s.indexer.LastSyncTime(req.SessionID).UnixMilli(),
		Scope:      scope,
	}, nil
}

// planFilters converts SearchFilters to bitmap operations.
func (s *SearchEngine) planFilters(filters *types.SearchFilters, query string) *roaring.Bitmap {
	// Start with all documents
	result := s.indexer.AllDocIDs()

	if filters == nil && query == "" {
		return result
	}

	// Apply structured filters
	if filters != nil {
		if filters.Host != "" {
			if bm := s.indexer.GetBitmapForHost(strings.ToLower(filters.Host)); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New() // No matches
			}
		}

		if filters.Method != "" {
			if bm := s.indexer.GetBitmapForMethod(filters.Method); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.Status != 0 {
			if bm := s.indexer.GetBitmapForStatus(filters.Status); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.HTTPVersion != "" {
			if bm := s.indexer.GetBitmapForHTTPVersion(filters.HTTPVersion); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.ProcessName != "" {
			if bm := s.indexer.GetBitmapForProcessName(filters.ProcessName); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.PID != 0 {
			if bm := s.indexer.GetBitmapForPID(filters.PID); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.HeaderName != "" {
			if bm := s.indexer.GetBitmapForHeaderName(strings.ToLower(filters.HeaderName)); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.TLSConnectionID != "" {
			if bm := s.indexer.GetBitmapForTLSConnection(filters.TLSConnectionID); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.JA3 != "" {
			if bm := s.indexer.GetBitmapForJA3(filters.JA3); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}

		if filters.JA4 != "" {
			if bm := s.indexer.GetBitmapForJA4(filters.JA4); bm != nil {
				result = roaring.And(result, bm)
			} else {
				return roaring.New()
			}
		}
	}

	// Apply free text query: OR across URL, header, body token indexes per token, AND across tokens
	if query != "" {
		queryTokens := indexer.Tokenize(query)
		for _, token := range queryTokens {
			union := roaring.New()

			if bm := s.indexer.GetBitmapForToken(token); bm != nil {
				union.Or(bm)
			}
			if bm := s.indexer.GetBitmapForHeaderToken(token); bm != nil {
				union.Or(bm)
			}
			if s.indexer.BodyIndexEnabled() {
				if bm := s.indexer.GetBitmapForBodyToken(token); bm != nil {
					union.Or(bm)
				}
			}

			result = roaring.And(result, union)
		}
	}

	// PathContains, URLContains, HeaderContains, BodyContains require post-filtering

	return result
}

// postFilterResult holds the filtered bitmap and cache statistics.
type postFilterResult struct {
	bitmap          *roaring.Bitmap
	bodyCacheHits   int
	bodyCacheMisses int
}

// applyPostFilters applies time-based and text-contains filters that require metadata access.
func (s *SearchEngine) applyPostFilters(candidates *roaring.Bitmap, filters *types.SearchFilters) postFilterResult {
	if filters == nil {
		return postFilterResult{bitmap: candidates}
	}

	// Determine time bounds
	var sinceMs, untilMs int64
	if filters.TimeWindowMs > 0 {
		now := time.Now().UnixMilli()
		sinceMs = now - filters.TimeWindowMs
		untilMs = now
	} else {
		sinceMs = filters.SinceMs
		untilMs = filters.UntilMs
	}

	needsFiltering := sinceMs > 0 || untilMs > 0 ||
		filters.PathContains != "" || filters.URLContains != "" ||
		filters.HeaderContains != "" || filters.BodyContains != ""
	if !needsFiltering {
		return postFilterResult{bitmap: candidates}
	}

	headerContainsLower := strings.ToLower(filters.HeaderContains)
	bodyContainsLower := strings.ToLower(filters.BodyContains)

	result := roaring.New()
	iter := candidates.Iterator()
	var bodyCacheHits, bodyCacheMisses int

	for iter.HasNext() {
		docID := iter.Next()
		meta := s.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}

		// Time filter
		if sinceMs > 0 && meta.TsMs < sinceMs {
			continue
		}
		if untilMs > 0 && meta.TsMs > untilMs {
			continue
		}

		// PathContains filter
		if filters.PathContains != "" {
			if !strings.Contains(strings.ToLower(meta.Path), strings.ToLower(filters.PathContains)) {
				continue
			}
		}

		// URLContains filter
		if filters.URLContains != "" {
			if !strings.Contains(strings.ToLower(meta.URL), strings.ToLower(filters.URLContains)) {
				continue
			}
		}

		// HeaderContains filter: case-insensitive substring on "name: value"
		if filters.HeaderContains != "" {
			found := false
			for _, hv := range meta.HeaderValues {
				field := strings.ToLower(hv.Name + ": " + hv.Value)
				if strings.Contains(field, headerContainsLower) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// BodyContains filter: fetch from cache, decode base64, substring match
		if filters.BodyContains != "" {
			if s.cache == nil {
				bodyCacheMisses++
				continue
			}
			entry, ok := s.cache.Get(meta.EntryID)
			if !ok {
				bodyCacheMisses++
				continue
			}
			bodyCacheHits++
			if !bodyContainsMatch(entry, bodyContainsLower) {
				continue
			}
		}

		result.Add(docID)
	}

	return postFilterResult{
		bitmap:          result,
		bodyCacheHits:   bodyCacheHits,
		bodyCacheMisses: bodyCacheMisses,
	}
}

// bodyContainsMatch checks if the decoded body text of request or response contains the substring.
func bodyContainsMatch(entry *client.SessionEntry, needle string) bool {
	// Check request body
	if entry.Request.Body != nil && *entry.Request.Body != "" {
		if decoded, err := base64.StdEncoding.DecodeString(*entry.Request.Body); err == nil {
			if strings.Contains(strings.ToLower(string(decoded)), needle) {
				return true
			}
		}
	}

	// Check response body
	if entry.Response != nil && entry.Response.Body != nil && *entry.Response.Body != "" {
		if decoded, err := base64.StdEncoding.DecodeString(*entry.Response.Body); err == nil {
			if strings.Contains(strings.ToLower(string(decoded)), needle) {
				return true
			}
		}
	}

	return false
}

// scoreResults applies ranking heuristics to produce scored results.
func (s *SearchEngine) scoreResults(docIDs []uint32, req *types.SearchRequest) []types.SearchResult {
	results := make([]types.SearchResult, 0, len(docIDs))

	// Parse query tokens for scoring
	var queryTokens []string
	if req.Query != "" {
		queryTokens = indexer.Tokenize(req.Query)
	}

	// Find time range for recency scoring
	var minTs, maxTs int64
	for _, docID := range docIDs {
		meta := s.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}
		if minTs == 0 || meta.TsMs < minTs {
			minTs = meta.TsMs
		}
		if meta.TsMs > maxTs {
			maxTs = meta.TsMs
		}
	}
	timeRange := maxTs - minTs
	if timeRange == 0 {
		timeRange = 1 // Avoid division by zero
	}

	bodyIndexEnabled := s.indexer.BodyIndexEnabled()

	for _, docID := range docIDs {
		meta := s.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}

		var score float64
		var highlights []string
		var matchedIn []string

		// Cross-index token scoring
		if len(queryTokens) > 0 {
			totalTokens := float64(len(queryTokens))

			// URL token matches (weight: 0.4)
			urlTokens := indexer.TokenizeURL(meta.URL)
			urlTokenSet := make(map[string]struct{}, len(urlTokens))
			for _, t := range urlTokens {
				urlTokenSet[t] = struct{}{}
			}
			urlMatches := 0
			for _, qt := range queryTokens {
				if _, exists := urlTokenSet[qt]; exists {
					urlMatches++
					highlights = append(highlights, qt)
				}
			}
			if urlMatches > 0 {
				score += float64(urlMatches) / totalTokens * 0.4
				matchedIn = appendUnique(matchedIn, "url")
			}

			// Header token matches (weight: 0.3)
			headerTokens := indexer.TokenizeHeaders(meta.HeaderValues)
			headerTokenSet := make(map[string]struct{}, len(headerTokens))
			for _, t := range headerTokens {
				headerTokenSet[t] = struct{}{}
			}
			headerMatches := 0
			for _, qt := range queryTokens {
				if _, exists := headerTokenSet[qt]; exists {
					headerMatches++
				}
			}
			if headerMatches > 0 {
				score += float64(headerMatches) / totalTokens * 0.3
				matchedIn = appendUnique(matchedIn, "header")
			}

			// Body token matches (weight: 0.2) - only check bitmap if body indexing enabled
			if bodyIndexEnabled {
				bodyMatches := 0
				for _, qt := range queryTokens {
					if bm := s.indexer.GetBitmapForBodyToken(qt); bm != nil && bm.Contains(docID) {
						bodyMatches++
					}
				}
				if bodyMatches > 0 {
					score += float64(bodyMatches) / totalTokens * 0.2
					matchedIn = appendUnique(matchedIn, "body")
				}
			}
		}

		// Method match boost
		if req.Filters != nil && req.Filters.Method != "" && meta.Method == req.Filters.Method {
			score += 0.1
		}

		// Recency boost (newer = higher score, weight: 0.3)
		if timeRange > 0 {
			recencyScore := float64(meta.TsMs-minTs) / float64(timeRange)
			score += recencyScore * 0.3
		}

		// Header name overlap if filter specified
		if req.Filters != nil && req.Filters.HeaderName != "" {
			headerLower := strings.ToLower(req.Filters.HeaderName)
			for _, h := range meta.HeaderNamesLower {
				if h == headerLower {
					score += 0.2
					break
				}
			}
		}

		// Base score for all results
		score += 0.1

		result := types.SearchResult{
			Summary:    meta.ToSummary(),
			Score:      score,
			Highlights: highlights,
		}
		if len(matchedIn) > 0 {
			result.MatchedIn = matchedIn
		}

		results = append(results, result)
	}

	return results
}

// appendUnique appends a value to a slice if it's not already present.
func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
