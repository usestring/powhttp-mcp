package catalog

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/jsonschema"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// DescribeEngine generates detailed endpoint descriptions.
type DescribeEngine struct {
	indexer *indexer.Indexer
	client  *client.Client
	cache   *cache.EntryCache
	config  *config.Config
	store   *ClusterStore
}

// NewDescribeEngine creates a new DescribeEngine.
func NewDescribeEngine(idx *indexer.Indexer, c *client.Client, cache *cache.EntryCache, cfg *config.Config, store *ClusterStore) *DescribeEngine {
	return &DescribeEngine{
		indexer: idx,
		client:  c,
		cache:   cache,
		config:  cfg,
		store:   store,
	}
}

// Describe generates a detailed description for an endpoint cluster.
func (d *DescribeEngine) Describe(ctx context.Context, req *types.DescribeRequest) (*types.EndpointDescription, error) {
	// Get cluster from store
	stored, ok := d.store.GetCluster(req.ClusterID)
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", req.ClusterID)
	}

	// Apply defaults
	maxExamples := req.MaxExamples
	if maxExamples <= 0 {
		maxExamples = 5
	}

	// Fetch entries for analysis
	entries, err := d.fetchEntries(ctx, req.SessionID, stored.EntryIDs, maxExamples)
	if err != nil {
		return nil, fmt.Errorf("fetching entries: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries found for cluster: %s", req.ClusterID)
	}

	// Analyze entries
	headers := analyzeHeaders(entries)
	authSignals := detectAuthSignals(entries)
	queryKeys := analyzeQueryKeys(entries)
	bodySchema := extractRequestBodySchema(entries)

	// Build examples
	examples := make([]types.ExampleEntry, 0, len(entries))
	for _, entry := range entries {
		meta := d.indexer.GetMetaByEntryID(entry.ID)
		if meta == nil {
			continue
		}
		examples = append(examples, types.ExampleEntry{
			EntryID: entry.ID,
			Summary: meta.ToSummary(),
		})
	}

	return &types.EndpointDescription{
		ClusterID:         req.ClusterID,
		Host:              stored.Cluster.Host,
		Method:            stored.Cluster.Method,
		PathTemplate:      stored.Cluster.PathTemplate,
		Count:             stored.Cluster.Count,
		TypicalHeaders:    headers,
		AuthSignals:       authSignals,
		QueryKeys:         queryKeys,
		RequestBodySchema: bodySchema,
		Examples:          examples,
	}, nil
}

// fetchEntries retrieves entries from cache or API.
func (d *DescribeEngine) fetchEntries(ctx context.Context, sessionID string, entryIDs []string, limit int) ([]*client.SessionEntry, error) {
	// Limit the number of entries to fetch
	toFetch := entryIDs
	if len(toFetch) > limit {
		toFetch = toFetch[:limit]
	}

	entries := make([]*client.SessionEntry, 0, len(toFetch))
	for _, entryID := range toFetch {
		// Try cache first
		if d.cache != nil {
			if cached, ok := d.cache.Get(entryID); ok {
				entries = append(entries, cached)
				continue
			}
		}

		// Fetch from API
		entry, err := d.client.GetEntry(ctx, sessionID, entryID)
		if err != nil {
			// Skip entries that can't be fetched
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// analyzeHeaders computes header frequencies across entries.
func analyzeHeaders(entries []*client.SessionEntry) []types.HeaderFrequency {
	if len(entries) == 0 {
		return nil
	}

	headerCounts := make(map[string]int)

	for _, entry := range entries {
		seen := make(map[string]bool)
		for _, header := range entry.Request.Headers {
			if len(header) < 2 {
				continue
			}
			name := strings.ToLower(header[0])
			if !seen[name] {
				headerCounts[name]++
				seen[name] = true
			}
		}
	}

	total := float64(len(entries))
	frequencies := make([]types.HeaderFrequency, 0, len(headerCounts))

	for name, count := range headerCounts {
		frequencies = append(frequencies, types.HeaderFrequency{
			Name:      name,
			Frequency: float64(count) / total,
		})
	}

	// Sort by frequency descending
	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].Frequency > frequencies[j].Frequency
	})

	// Return top headers (limit to reasonable number)
	if len(frequencies) > 20 {
		frequencies = frequencies[:20]
	}

	return frequencies
}

// detectAuthSignals looks for authentication patterns.
func detectAuthSignals(entries []*client.SessionEntry) types.AuthSignals {
	signals := types.AuthSignals{
		CustomAuthHeaders: make([]string, 0),
	}

	customAuthSet := make(map[string]bool)

	for _, entry := range entries {
		for _, header := range entry.Request.Headers {
			if len(header) < 2 {
				continue
			}
			name := strings.ToLower(header[0])
			value := header[1]

			switch name {
			case "cookie":
				signals.CookiesPresent = true
			case "authorization":
				if strings.HasPrefix(strings.ToLower(value), "bearer ") {
					signals.BearerPresent = true
				}
			case "x-api-key", "x-auth-token", "x-access-token":
				if !customAuthSet[name] {
					signals.CustomAuthHeaders = append(signals.CustomAuthHeaders, name)
					customAuthSet[name] = true
				}
			}
		}
	}

	return signals
}

// analyzeQueryKeys categorizes query parameters as stable or volatile.
func analyzeQueryKeys(entries []*client.SessionEntry) types.QueryKeyAnalysis {
	analysis := types.QueryKeyAnalysis{
		Stable:   make([]string, 0),
		Volatile: make([]string, 0),
	}

	if len(entries) == 0 {
		return analysis
	}

	// Track key presence and value variation
	keyCounts := make(map[string]int)
	keyValues := make(map[string]map[string]bool)

	for _, entry := range entries {
		parsed, err := url.Parse(entry.URL)
		if err != nil {
			continue
		}

		for key, values := range parsed.Query() {
			keyCounts[key]++
			if keyValues[key] == nil {
				keyValues[key] = make(map[string]bool)
			}
			for _, v := range values {
				keyValues[key][v] = true
			}
		}
	}

	total := len(entries)
	volatilePatterns := []string{"timestamp", "ts", "t", "time", "nonce", "rand", "random", "_"}

	for key, count := range keyCounts {
		// Check if key matches volatile patterns
		isVolatile := false
		keyLower := strings.ToLower(key)
		for _, pattern := range volatilePatterns {
			if keyLower == pattern || strings.HasPrefix(keyLower, pattern) {
				isVolatile = true
				break
			}
		}

		// Check if values vary across 80%+ of entries
		if !isVolatile && len(keyValues[key]) > 0 {
			uniqueRatio := float64(len(keyValues[key])) / float64(count)
			if uniqueRatio > 0.8 {
				isVolatile = true
			}
		}

		// Key is stable if present in most requests and not volatile
		presenceRatio := float64(count) / float64(total)
		if presenceRatio >= 0.5 && !isVolatile {
			analysis.Stable = append(analysis.Stable, key)
		} else if isVolatile {
			analysis.Volatile = append(analysis.Volatile, key)
		}
	}

	sort.Strings(analysis.Stable)
	sort.Strings(analysis.Volatile)

	return analysis
}

// extractRequestBodySchema infers JSON Schema from request bodies.
func extractRequestBodySchema(entries []*client.SessionEntry) *types.RequestBodySchema {
	var samples [][]byte
	var contentType string

	for _, entry := range entries {
		if entry.Request.Body == nil {
			continue
		}

		// Check content type
		ct := ""
		for _, header := range entry.Request.Headers {
			if len(header) >= 2 && strings.ToLower(header[0]) == "content-type" {
				ct = strings.ToLower(header[1])
				break
			}
		}

		if !strings.Contains(ct, "json") {
			continue
		}

		// Decode body
		bodyBytes, err := base64.StdEncoding.DecodeString(*entry.Request.Body)
		if err != nil {
			continue
		}

		samples = append(samples, bodyBytes)
		if contentType == "" {
			contentType = ct
		}
	}

	if len(samples) == 0 {
		return nil
	}

	inferred, err := jsonschema.Infer(samples...)
	if err != nil || inferred == nil {
		return nil
	}

	return &types.RequestBodySchema{
		Schema:      inferred.Schema,
		SampleCount: inferred.SampleCount,
		AllMatch:    inferred.AllMatch,
		ContentType: contentType,
	}
}
