package search

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// --- helpers ---

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func newTestCache() *cache.EntryCache {
	c, _ := cache.NewEntryCache(128)
	return c
}

// testFixture builds an indexer with pre-seeded entries and a search engine.
type testFixture struct {
	idx    *indexer.Indexer
	cache  *cache.EntryCache
	engine *SearchEngine
}

func newFixture(bodyIndex bool) *testFixture {
	cfg := &config.Config{
		IndexBody:         bodyIndex,
		IndexBodyMaxBytes: 65536,
	}
	ec := newTestCache()
	idx := indexer.New(nil, ec, cfg)

	return &testFixture{
		idx:    idx,
		cache:  ec,
		engine: New(idx, ec, cfg),
	}
}

func (f *testFixture) addEntry(e *client.SessionEntry) uint32 {
	return f.idx.Index(e)
}

func makeEntry(id, url, method string, status int, tsMs int64) *client.SessionEntry {
	return &client.SessionEntry{
		ID:          id,
		URL:         url,
		HTTPVersion: "h2",
		Request: client.Request{
			Method: strPtr(method),
			Headers: client.Headers{
				{"Host", "example.com"},
			},
		},
		Response: &client.Response{
			StatusCode: intPtr(status),
			Headers:    client.Headers{},
		},
		Timings: client.Timings{StartedAt: tsMs},
	}
}

func makeEntryWithHeaders(id, url, method string, status int, tsMs int64, reqHeaders client.Headers) *client.SessionEntry {
	e := makeEntry(id, url, method, status, tsMs)
	e.Request.Headers = reqHeaders
	return e
}

func makeEntryWithBody(id, url, method string, status int, tsMs int64, respContentType, respBody string) *client.SessionEntry {
	e := makeEntry(id, url, method, status, tsMs)
	encoded := base64.StdEncoding.EncodeToString([]byte(respBody))
	e.Response = &client.Response{
		StatusCode: intPtr(status),
		Headers:    client.Headers{{"Content-Type", respContentType}},
		Body:       &encoded,
	}
	return e
}

// --- planFilters tests ---

func TestPlanFilters_NoFilters(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/p1", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://b.com/p2", "POST", 201, 2000))

	result := f.engine.planFilters(nil, "")
	assert.Equal(t, uint64(2), result.GetCardinality())
}

func TestPlanFilters_HostFilter(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/a", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/b", "GET", 200, 2000))
	f.addEntry(makeEntry("e3", "https://other.com/c", "GET", 200, 3000))

	result := f.engine.planFilters(&types.SearchFilters{Host: "api.example.com"}, "")
	assert.Equal(t, uint64(2), result.GetCardinality())
}

func TestPlanFilters_HostFilter_Wildcard(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://example.com/a", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/b", "GET", 200, 2000))
	f.addEntry(makeEntry("e3", "https://other.com/c", "GET", 200, 3000))

	result := f.engine.planFilters(&types.SearchFilters{Host: "*.example.com"}, "")
	assert.Equal(t, uint64(2), result.GetCardinality())
}

func TestPlanFilters_HostFilter_NoMatch(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))

	result := f.engine.planFilters(&types.SearchFilters{Host: "nonexistent.com"}, "")
	assert.Equal(t, uint64(0), result.GetCardinality())
}

func TestPlanFilters_MethodFilter(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/", "POST", 201, 2000))
	f.addEntry(makeEntry("e3", "https://a.com/", "GET", 200, 3000))

	result := f.engine.planFilters(&types.SearchFilters{Method: "GET"}, "")
	assert.Equal(t, uint64(2), result.GetCardinality())
}

func TestPlanFilters_StatusFilter(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/", "GET", 404, 2000))

	result := f.engine.planFilters(&types.SearchFilters{Status: 404}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_CombinedFilters(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/", "POST", 200, 2000))
	f.addEntry(makeEntry("e3", "https://other.com/", "GET", 200, 3000))

	// Host AND Method filter
	result := f.engine.planFilters(&types.SearchFilters{
		Host:   "api.example.com",
		Method: "GET",
	}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_ProcessNameFilter(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
	e1.Process = &client.ProcessInfo{PID: 1, Name: strPtr("Chrome")}
	f.addEntry(e1)

	e2 := makeEntry("e2", "https://a.com/", "GET", 200, 2000)
	e2.Process = &client.ProcessInfo{PID: 2, Name: strPtr("python")}
	f.addEntry(e2)

	result := f.engine.planFilters(&types.SearchFilters{ProcessName: "Chrome"}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_PIDFilter(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
	e1.Process = &client.ProcessInfo{PID: 1234}
	f.addEntry(e1)

	e2 := makeEntry("e2", "https://a.com/", "GET", 200, 2000)
	e2.Process = &client.ProcessInfo{PID: 5678}
	f.addEntry(e2)

	result := f.engine.planFilters(&types.SearchFilters{PID: 1234}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_HeaderNameFilter(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntryWithHeaders("e1", "https://a.com/", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer tok"},
	})
	f.addEntry(e1)

	e2 := makeEntryWithHeaders("e2", "https://a.com/", "GET", 200, 2000, client.Headers{
		{"Content-Type", "text/html"},
	})
	f.addEntry(e2)

	result := f.engine.planFilters(&types.SearchFilters{HeaderName: "authorization"}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_TLSFilters(t *testing.T) {
	f := newFixture(false)

	e := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
	e.TLS = client.TLSInfo{
		ConnectionID: strPtr("tls-1"),
		JA3:          &client.JA3Fingerprint{Hash: "j3hash"},
		JA4:          &client.JA4Fingerprint{Hashed: "j4hash"},
	}
	f.addEntry(e)
	f.addEntry(makeEntry("e2", "https://a.com/", "GET", 200, 2000))

	assert.Equal(t, uint64(1), f.engine.planFilters(&types.SearchFilters{TLSConnectionID: "tls-1"}, "").GetCardinality())
	assert.Equal(t, uint64(1), f.engine.planFilters(&types.SearchFilters{JA3: "j3hash"}, "").GetCardinality())
	assert.Equal(t, uint64(1), f.engine.planFilters(&types.SearchFilters{JA4: "j4hash"}, "").GetCardinality())
	assert.Equal(t, uint64(0), f.engine.planFilters(&types.SearchFilters{JA4: "nope"}, "").GetCardinality())
}

func TestPlanFilters_HTTPVersionFilter(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000)) // h2

	e2 := makeEntry("e2", "https://a.com/", "GET", 200, 2000)
	e2.HTTPVersion = "HTTP/1.1"
	f.addEntry(e2)

	result := f.engine.planFilters(&types.SearchFilters{HTTPVersion: "h2"}, "")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_QueryTokenSearch(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/users/search?q=hello", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/products", "GET", 200, 2000))

	// "users" matches e1 URL
	result := f.engine.planFilters(nil, "users")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_QueryTokenSearch_ANDLogic(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/users/search", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/products/search", "GET", 200, 2000))

	// "users" AND "search" - only e1 has both in URL
	result := f.engine.planFilters(nil, "users search")
	assert.Equal(t, uint64(1), result.GetCardinality())

	// "search" matches both
	result = f.engine.planFilters(nil, "search")
	assert.Equal(t, uint64(2), result.GetCardinality())
}

func TestPlanFilters_QueryTokenSearch_CrossIndex(t *testing.T) {
	f := newFixture(false)

	// e1 has "bearer" in header, not URL
	e1 := makeEntryWithHeaders("e1", "https://a.com/api", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer secret-token"},
	})
	f.addEntry(e1)

	// "bearer" should match via header token index
	result := f.engine.planFilters(nil, "bearer")
	assert.Equal(t, uint64(1), result.GetCardinality())
}

func TestPlanFilters_QueryWithBodyIndex(t *testing.T) {
	f := newFixture(true) // body indexing enabled

	e1 := makeEntryWithBody("e1", "https://a.com/api", "GET", 200, 1000, "application/json", `{"product":"widget"}`)
	f.addEntry(e1)

	// "widget" only in response body
	result := f.engine.planFilters(nil, "widget")
	assert.Equal(t, uint64(1), result.GetCardinality())

	// "nonexistent" nowhere
	result = f.engine.planFilters(nil, "nonexistent")
	assert.Equal(t, uint64(0), result.GetCardinality())
}

// --- applyPostFilters tests ---

func TestApplyPostFilters_NilFilters(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))

	all := f.idx.AllDocIDs()
	result := f.engine.applyPostFilters(all, nil)
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_TimeRange(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/a", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/b", "GET", 200, 2000))
	f.addEntry(makeEntry("e3", "https://a.com/c", "GET", 200, 3000))

	all := f.idx.AllDocIDs()
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		SinceMs: 1500,
		UntilMs: 2500,
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_SinceOnly(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/a", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/b", "GET", 200, 2000))
	f.addEntry(makeEntry("e3", "https://a.com/c", "GET", 200, 3000))

	all := f.idx.AllDocIDs()
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		SinceMs: 2000,
	})
	assert.Equal(t, uint64(2), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_PathContains(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/api/users", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/api/products", "GET", 200, 2000))

	all := f.idx.AllDocIDs()
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		PathContains: "users",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_URLContains(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/v1/data", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://cdn.other.com/assets/img.png", "GET", 200, 2000))

	all := f.idx.AllDocIDs()
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		URLContains: "example.com",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_HeaderContains(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntryWithHeaders("e1", "https://a.com/", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer my-token"},
	})
	e2 := makeEntryWithHeaders("e2", "https://a.com/", "GET", 200, 2000, client.Headers{
		{"Content-Type", "text/html"},
	})
	f.addEntry(e1)
	f.addEntry(e2)

	all := f.idx.AllDocIDs()

	// Case-insensitive substring match on "name: value"
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "bearer",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())

	// Match header name
	result = f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "authorization",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())

	// Match full field "content-type: text/html"
	result = f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "content-type: text",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())

	// No match
	result = f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "x-api-key",
	})
	assert.Equal(t, uint64(0), result.bitmap.GetCardinality())
}

func TestApplyPostFilters_BodyContains(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntryWithBody("e1", "https://a.com/x", "GET", 200, 1000, "application/json", `{"error":"UNIQUE_ERR_XYZ_42"}`)
	e2 := makeEntryWithBody("e2", "https://b.com/y", "GET", 200, 2000, "application/json", `{"data":"all good"}`)
	f.addEntry(e1)
	f.addEntry(e2)

	// Verify both entries are in cache
	_, ok1 := f.cache.Get("e1")
	_, ok2 := f.cache.Get("e2")
	require.True(t, ok1, "e1 should be in cache")
	require.True(t, ok2, "e2 should be in cache")

	all := f.idx.AllDocIDs()
	require.Equal(t, uint64(2), all.GetCardinality())

	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		BodyContains: "UNIQUE_ERR_XYZ_42",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality(), "only e1 body should match")
	assert.Equal(t, 2, result.bodyCacheHits, "both entries should be found in cache")
	assert.Equal(t, 0, result.bodyCacheMisses)
}

func TestApplyPostFilters_BodyContains_NilCache(t *testing.T) {
	cfg := &config.Config{}
	idx := indexer.New(nil, nil, cfg) // nil cache
	engine := New(idx, nil, cfg)      // nil cache

	e := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
	idx.Index(e)

	all := idx.AllDocIDs()
	result := engine.applyPostFilters(all, &types.SearchFilters{
		BodyContains: "test",
	})
	assert.Equal(t, uint64(0), result.bitmap.GetCardinality())
	assert.Equal(t, 0, result.bodyCacheHits)
	assert.Equal(t, 1, result.bodyCacheMisses)
}

func TestApplyPostFilters_BodyContains_CacheMiss(t *testing.T) {
	f := newFixture(false)

	// Index entry without putting it in cache
	e := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
	f.idx.Index(e) // This uses the indexer's cache, so entry IS cached...

	// Let's create a scenario where cache doesn't have the entry
	// by creating a fresh cache and a new engine
	freshCache := newTestCache()
	engine := New(f.idx, freshCache, &config.Config{}) // fresh cache without entries

	all := f.idx.AllDocIDs()
	result := engine.applyPostFilters(all, &types.SearchFilters{
		BodyContains: "test",
	})
	assert.Equal(t, uint64(0), result.bitmap.GetCardinality())
	assert.Equal(t, 0, result.bodyCacheHits)
	assert.Equal(t, 1, result.bodyCacheMisses)
}

func TestApplyPostFilters_NoFilteringNeeded(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))

	all := f.idx.AllDocIDs()

	// Empty filters struct with no active fields
	result := f.engine.applyPostFilters(all, &types.SearchFilters{})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

// --- bodyContainsMatch tests ---

func TestBodyContainsMatch(t *testing.T) {
	tests := []struct {
		name     string
		entry    *client.SessionEntry
		needle   string
		expected bool
	}{
		{
			"matches response body",
			makeEntryWithBody("e1", "https://a.com/", "GET", 200, 1000, "application/json", `{"error":"access denied"}`),
			"access denied",
			true,
		},
		{
			"case insensitive",
			makeEntryWithBody("e1", "https://a.com/", "GET", 200, 1000, "application/json", `{"Error":"ACCESS DENIED"}`),
			"access denied",
			true,
		},
		{
			"no match",
			makeEntryWithBody("e1", "https://a.com/", "GET", 200, 1000, "application/json", `{"data":"ok"}`),
			"error",
			false,
		},
		{
			"matches request body",
			func() *client.SessionEntry {
				e := makeEntry("e1", "https://a.com/", "POST", 200, 1000)
				reqBody := base64.StdEncoding.EncodeToString([]byte(`{"query":"search term"}`))
				e.Request.Body = &reqBody
				return e
			}(),
			"search term",
			true,
		},
		{
			"nil response body",
			makeEntry("e1", "https://a.com/", "GET", 200, 1000),
			"test",
			false,
		},
		{
			"nil response",
			func() *client.SessionEntry {
				e := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
				e.Response = nil
				return e
			}(),
			"test",
			false,
		},
		{
			"empty request body",
			func() *client.SessionEntry {
				e := makeEntry("e1", "https://a.com/", "GET", 200, 1000)
				empty := ""
				e.Request.Body = &empty
				return e
			}(),
			"test",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, bodyContainsMatch(tt.entry, tt.needle))
		})
	}
}

// --- scoreResults tests ---

func TestScoreResults_BaseScore(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{}
	results := f.engine.scoreResults(docIDs, req)

	require.Len(t, results, 1)
	// Base score = 0.1, recency = 0.3 (single entry gets max recency)
	assert.Greater(t, results[0].Score, 0.0)
	assert.Equal(t, "e1", results[0].Summary.EntryID)
}

func TestScoreResults_RecencyScoring(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/a", "GET", 200, 1000)) // oldest
	f.addEntry(makeEntry("e2", "https://a.com/b", "GET", 200, 2000))
	f.addEntry(makeEntry("e3", "https://a.com/c", "GET", 200, 3000)) // newest

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{}
	results := f.engine.scoreResults(docIDs, req)

	require.Len(t, results, 3)

	// Find scores by entry ID
	scores := map[string]float64{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
	}

	// Newer entries should score higher due to recency boost
	assert.Greater(t, scores["e3"], scores["e1"])
}

func TestScoreResults_QueryTokenMatching(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/users", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://api.example.com/products", "GET", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{Query: "users"}
	results := f.engine.scoreResults(docIDs, req)

	require.Len(t, results, 2)

	scores := map[string]float64{}
	matchedIn := map[string][]string{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
		matchedIn[r.Summary.EntryID] = r.MatchedIn
	}

	// e1 should score higher because "users" appears in its URL
	assert.Greater(t, scores["e1"], scores["e2"])
	assert.Contains(t, matchedIn["e1"], "url")
}

func TestScoreResults_CrossIndexMatching(t *testing.T) {
	f := newFixture(false)

	// e1 has "bearer" in header
	e1 := makeEntryWithHeaders("e1", "https://a.com/api", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer tok123"},
	})
	f.addEntry(e1)

	// e2 has no auth header
	f.addEntry(makeEntry("e2", "https://a.com/public", "GET", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{Query: "bearer"}
	results := f.engine.scoreResults(docIDs, req)

	scores := map[string]float64{}
	matchedIn := map[string][]string{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
		matchedIn[r.Summary.EntryID] = r.MatchedIn
	}

	assert.Greater(t, scores["e1"], scores["e2"])
	assert.Contains(t, matchedIn["e1"], "header")
}

func TestScoreResults_BodyMatching(t *testing.T) {
	f := newFixture(true) // body indexing enabled

	e1 := makeEntryWithBody("e1", "https://a.com/api", "GET", 200, 1000, "application/json", `{"product":"widget"}`)
	f.addEntry(e1)
	f.addEntry(makeEntry("e2", "https://a.com/other", "GET", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{Query: "widget"}
	results := f.engine.scoreResults(docIDs, req)

	scores := map[string]float64{}
	matchedIn := map[string][]string{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
		matchedIn[r.Summary.EntryID] = r.MatchedIn
	}

	assert.Greater(t, scores["e1"], scores["e2"])
	assert.Contains(t, matchedIn["e1"], "body")
}

func TestScoreResults_MethodMatchBoost(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/api", "GET", 200, 1000))
	f.addEntry(makeEntry("e2", "https://a.com/api", "POST", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{
		Filters: &types.SearchFilters{Method: "GET"},
	}
	results := f.engine.scoreResults(docIDs, req)

	scores := map[string]float64{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
	}

	// GET entry should get method boost
	assert.Greater(t, scores["e1"], scores["e2"])
}

func TestScoreResults_HeaderNameBoost(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntryWithHeaders("e1", "https://a.com/", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer tok"},
	})
	e2 := makeEntryWithHeaders("e2", "https://a.com/", "GET", 200, 1000, client.Headers{
		{"Content-Type", "text/html"},
	})
	f.addEntry(e1)
	f.addEntry(e2)

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{
		Filters: &types.SearchFilters{HeaderName: "authorization"},
	}
	results := f.engine.scoreResults(docIDs, req)

	scores := map[string]float64{}
	for _, r := range results {
		scores[r.Summary.EntryID] = r.Score
	}

	assert.Greater(t, scores["e1"], scores["e2"])
}

func TestScoreResults_Highlights(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://api.example.com/users/search", "GET", 200, 1000))

	docIDs := f.idx.AllDocIDs().ToArray()
	req := &types.SearchRequest{Query: "users search"}
	results := f.engine.scoreResults(docIDs, req)

	require.Len(t, results, 1)
	assert.Contains(t, results[0].Highlights, "users")
	assert.Contains(t, results[0].Highlights, "search")
}

func TestScoreResults_EmptyDocIDs(t *testing.T) {
	f := newFixture(false)

	results := f.engine.scoreResults(nil, &types.SearchRequest{})
	assert.Empty(t, results)

	results = f.engine.scoreResults([]uint32{}, &types.SearchRequest{})
	assert.Empty(t, results)
}

func TestScoreResults_NilMeta(t *testing.T) {
	f := newFixture(false)
	f.addEntry(makeEntry("e1", "https://a.com/", "GET", 200, 1000))

	// Pass a docID that doesn't exist
	results := f.engine.scoreResults([]uint32{999}, &types.SearchRequest{})
	assert.Empty(t, results)
}

// --- appendUnique tests ---

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		val      string
		expected []string
	}{
		{"empty slice", nil, "url", []string{"url"}},
		{"new value", []string{"url"}, "header", []string{"url", "header"}},
		{"duplicate", []string{"url", "header"}, "url", []string{"url", "header"}},
		{"another new", []string{"url"}, "body", []string{"url", "body"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendUnique(tt.slice, tt.val)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Integration-level tests (without full Search() because it needs RefreshIfStale) ---

func TestSearchEngine_PlanAndScore_Integration(t *testing.T) {
	f := newFixture(false)

	// Create a realistic set of entries
	e1 := makeEntryWithHeaders("e1", "https://api.example.com/v1/users", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer admin-token"},
		{"Content-Type", "application/json"},
	})
	e2 := makeEntryWithHeaders("e2", "https://api.example.com/v1/products", "GET", 200, 2000, client.Headers{
		{"Content-Type", "application/json"},
	})
	e3 := makeEntryWithHeaders("e3", "https://cdn.example.com/assets/logo.png", "GET", 200, 3000, client.Headers{
		{"Content-Type", "image/png"},
	})
	e4 := makeEntryWithHeaders("e4", "https://api.example.com/v1/users", "POST", 201, 4000, client.Headers{
		{"Authorization", "Bearer admin-token"},
		{"Content-Type", "application/json"},
	})

	f.addEntry(e1)
	f.addEntry(e2)
	f.addEntry(e3)
	f.addEntry(e4)

	// Filter by host + query "users"
	candidates := f.engine.planFilters(&types.SearchFilters{
		Host: "api.example.com",
	}, "users")

	// Should match e1 and e4 (api.example.com + "users" in URL)
	assert.Equal(t, uint64(2), candidates.GetCardinality())

	// Score them
	req := &types.SearchRequest{
		Query:   "users",
		Filters: &types.SearchFilters{Host: "api.example.com"},
	}
	results := f.engine.scoreResults(candidates.ToArray(), req)
	assert.Len(t, results, 2)

	// Both should have "url" in matchedIn
	for _, r := range results {
		assert.Contains(t, r.MatchedIn, "url")
	}
}

func TestSearchEngine_PostFilterWithHeaderContains(t *testing.T) {
	f := newFixture(false)

	e1 := makeEntryWithHeaders("e1", "https://a.com/api", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer secret-token"},
	})
	e2 := makeEntryWithHeaders("e2", "https://a.com/api", "GET", 200, 2000, client.Headers{
		{"X-Api-Key", "key-123"},
	})
	e3 := makeEntryWithHeaders("e3", "https://a.com/public", "GET", 200, 3000, client.Headers{
		{"Content-Type", "text/html"},
	})

	f.addEntry(e1)
	f.addEntry(e2)
	f.addEntry(e3)

	all := f.idx.AllDocIDs()

	// Find entries with bearer auth
	result := f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "bearer",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())

	// Find entries with any API key
	result = f.engine.applyPostFilters(all, &types.SearchFilters{
		HeaderContains: "x-api-key",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}

func TestSearchEngine_CombinedIndexAndPostFilter(t *testing.T) {
	f := newFixture(false)

	// Two entries on same host, different headers
	e1 := makeEntryWithHeaders("e1", "https://api.example.com/data", "GET", 200, 1000, client.Headers{
		{"Authorization", "Bearer tok"},
	})
	e2 := makeEntryWithHeaders("e2", "https://api.example.com/data", "GET", 200, 2000, client.Headers{
		{"Content-Type", "text/html"},
	})
	e3 := makeEntryWithHeaders("e3", "https://other.com/data", "GET", 200, 3000, client.Headers{
		{"Authorization", "Bearer tok"},
	})

	f.addEntry(e1)
	f.addEntry(e2)
	f.addEntry(e3)

	// Step 1: Index filter (host)
	candidates := f.engine.planFilters(&types.SearchFilters{
		Host: "api.example.com",
	}, "")
	assert.Equal(t, uint64(2), candidates.GetCardinality())

	// Step 2: Post-filter (header contains)
	result := f.engine.applyPostFilters(candidates, &types.SearchFilters{
		Host:           "api.example.com",
		HeaderContains: "bearer",
	})
	assert.Equal(t, uint64(1), result.bitmap.GetCardinality())
}
