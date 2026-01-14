// Package search provides search capabilities over indexed HTTP entries.
package search

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/RoaringBitmap/roaring/v2"

	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// SearchEngine provides search capabilities over the indexer.
type SearchEngine struct {
	indexer *indexer.Indexer
}

// New creates a new SearchEngine.
func New(idx *indexer.Indexer) *SearchEngine {
	return &SearchEngine{indexer: idx}
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

	// Apply time filters (requires scanning metadata)
	candidates = s.applyTimeFilters(candidates, req.Filters)

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

	return &types.SearchResponse{
		Results:    paginated,
		TotalHint:  totalHint,
		SyncedAtMs: s.indexer.LastSyncTime(req.SessionID).UnixMilli(),
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

	// Apply free text query as token filter
	if query != "" {
		queryTokens := indexer.Tokenize(query)
		for _, token := range queryTokens {
			if bm := s.indexer.GetBitmapForToken(token); bm != nil {
				result = roaring.And(result, bm)
			}
		}
	}

	// PathContains and URLContains require post-filtering (done in applyTimeFilters)

	return result
}

// applyTimeFilters applies time-based and text-contains filters that require metadata access.
func (s *SearchEngine) applyTimeFilters(candidates *roaring.Bitmap, filters *types.SearchFilters) *roaring.Bitmap {
	if filters == nil {
		return candidates
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

	needsFiltering := sinceMs > 0 || untilMs > 0 || filters.PathContains != "" || filters.URLContains != ""
	if !needsFiltering {
		return candidates
	}

	result := roaring.New()
	iter := candidates.Iterator()

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

		result.Add(docID)
	}

	return result
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

	for _, docID := range docIDs {
		meta := s.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}

		var score float64
		var highlights []string

		// Token overlap scoring (0-1 range)
		if len(queryTokens) > 0 {
			urlTokens := indexer.TokenizeURL(meta.URL)
			tokenSet := make(map[string]struct{}, len(urlTokens))
			for _, t := range urlTokens {
				tokenSet[t] = struct{}{}
			}

			matches := 0
			for _, qt := range queryTokens {
				if _, exists := tokenSet[qt]; exists {
					matches++
					highlights = append(highlights, qt)
				}
			}
			score += float64(matches) / float64(len(queryTokens)) * 0.4
		}

		// Method match boost
		if req.Filters != nil && req.Filters.Method != "" && meta.Method == req.Filters.Method {
			score += 0.1
		}

		// Recency boost (newer = higher score)
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

		results = append(results, types.SearchResult{
			Summary:    meta.ToSummary(),
			Score:      score,
			Highlights: highlights,
		})
	}

	return results
}
