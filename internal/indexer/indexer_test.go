package indexer

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/pkg/client"
)

// newTestIndexer creates an Indexer suitable for unit testing (no client, no cache).
func newTestIndexer(cfg *config.Config) *Indexer {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return New(nil, nil, cfg)
}

// makeEntry builds a minimal SessionEntry for testing.
func makeEntry(id, url, method string, status int) *client.SessionEntry {
	return &client.SessionEntry{
		ID:          id,
		URL:         url,
		HTTPVersion: "h2",
		Request: client.Request{
			Method: strPtr(method),
			Headers: client.Headers{
				{"Host", extractHost(url)},
			},
		},
		Response: &client.Response{
			StatusCode: intPtr(status),
			Headers:    client.Headers{},
		},
		Timings: client.Timings{StartedAt: 1000},
	}
}

func TestIndex_AssignsSequentialDocIDs(t *testing.T) {
	idx := newTestIndexer(nil)

	id0 := idx.Index(makeEntry("e1", "https://a.com/p1", "GET", 200))
	id1 := idx.Index(makeEntry("e2", "https://b.com/p2", "POST", 201))

	assert.Equal(t, uint32(0), id0)
	assert.Equal(t, uint32(1), id1)
	assert.Equal(t, 2, idx.DocCount())
}

func TestIndex_Deduplicates(t *testing.T) {
	idx := newTestIndexer(nil)

	id0 := idx.Index(makeEntry("e1", "https://a.com/", "GET", 200))
	id1 := idx.Index(makeEntry("e1", "https://a.com/", "GET", 200)) // same entry ID

	assert.Equal(t, id0, id1)
	assert.Equal(t, 1, idx.DocCount())
}

func TestGetMeta(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://example.com/api", "GET", 200))

	meta := idx.GetMeta(0)
	require.NotNil(t, meta)
	assert.Equal(t, "e1", meta.EntryID)
	assert.Equal(t, "example.com", meta.Host)
	assert.Equal(t, "GET", meta.Method)

	// Out of bounds
	assert.Nil(t, idx.GetMeta(999))
}

func TestGetMetaByEntryID(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://example.com/api", "GET", 200))

	meta := idx.GetMetaByEntryID("e1")
	require.NotNil(t, meta)
	assert.Equal(t, "e1", meta.EntryID)

	assert.Nil(t, idx.GetMetaByEntryID("nonexistent"))
}

func TestAllDocIDs(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://a.com/", "GET", 200))
	idx.Index(makeEntry("e2", "https://b.com/", "POST", 201))
	idx.Index(makeEntry("e3", "https://c.com/", "PUT", 204))

	bm := idx.AllDocIDs()
	assert.Equal(t, uint64(3), bm.GetCardinality())
	assert.True(t, bm.Contains(0))
	assert.True(t, bm.Contains(1))
	assert.True(t, bm.Contains(2))
	assert.False(t, bm.Contains(3))
}

func TestBitmapIndexes_Host(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://example.com/a", "GET", 200))
	idx.Index(makeEntry("e2", "https://api.example.com/b", "GET", 200))
	idx.Index(makeEntry("e3", "https://www.example.com/c", "GET", 200))
	idx.Index(makeEntry("e4", "https://other.com/d", "GET", 200))
	idx.Index(makeEntry("e5", "https://notexample.com/e", "GET", 200))

	tests := []struct {
		name     string
		host     string
		wantNil  bool
		wantCard uint64
	}{
		{"exact match", "api.example.com", false, 1},
		{"exact match base", "example.com", false, 1},
		{"exact no match", "missing.com", true, 0},
		{"wildcard matches base and subdomains", "*.example.com", false, 3},
		{"wildcard excludes unrelated", "*.other.com", false, 1},
		{"wildcard no match", "*.missing.com", true, 0},
		{"wildcard does not match notexample.com", "*.example.com", false, 3},
		{"wildcard empty base", "*.", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := idx.GetBitmapForHost(tt.host)
			if tt.wantNil {
				assert.Nil(t, bm)
			} else {
				require.NotNil(t, bm)
				assert.Equal(t, tt.wantCard, bm.GetCardinality())
			}
		})
	}
}

func TestGetBitmapForHost_WildcardDoesNotMutateIndex(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://example.com/a", "GET", 200))
	idx.Index(makeEntry("e2", "https://api.example.com/b", "GET", 200))

	// Get the exact bitmap cardinality before wildcard call
	exactBm := idx.GetBitmapForHost("example.com")
	require.NotNil(t, exactBm)
	assert.Equal(t, uint64(1), exactBm.GetCardinality())

	// Wildcard call
	wildcardBm := idx.GetBitmapForHost("*.example.com")
	require.NotNil(t, wildcardBm)
	assert.Equal(t, uint64(2), wildcardBm.GetCardinality())

	// Verify exact bitmap was not mutated
	exactBm2 := idx.GetBitmapForHost("example.com")
	require.NotNil(t, exactBm2)
	assert.Equal(t, uint64(1), exactBm2.GetCardinality())
}

func TestBitmapIndexes_Method(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://a.com/", "GET", 200))
	idx.Index(makeEntry("e2", "https://a.com/", "POST", 201))
	idx.Index(makeEntry("e3", "https://a.com/", "GET", 200))

	bm := idx.GetBitmapForMethod("GET")
	require.NotNil(t, bm)
	assert.Equal(t, uint64(2), bm.GetCardinality())

	bm = idx.GetBitmapForMethod("POST")
	require.NotNil(t, bm)
	assert.Equal(t, uint64(1), bm.GetCardinality())

	assert.Nil(t, idx.GetBitmapForMethod("DELETE"))
}

func TestBitmapIndexes_Status(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://a.com/", "GET", 200))
	idx.Index(makeEntry("e2", "https://a.com/", "GET", 404))
	idx.Index(makeEntry("e3", "https://a.com/", "GET", 200))

	bm := idx.GetBitmapForStatus(200)
	require.NotNil(t, bm)
	assert.Equal(t, uint64(2), bm.GetCardinality())

	bm = idx.GetBitmapForStatus(404)
	require.NotNil(t, bm)
	assert.Equal(t, uint64(1), bm.GetCardinality())

	assert.Nil(t, idx.GetBitmapForStatus(500))
}

func TestBitmapIndexes_ProcessName(t *testing.T) {
	idx := newTestIndexer(nil)

	e1 := makeEntry("e1", "https://a.com/", "GET", 200)
	e1.Process = &client.ProcessInfo{PID: 1, Name: strPtr("Chrome")}

	e2 := makeEntry("e2", "https://a.com/", "GET", 200)
	e2.Process = &client.ProcessInfo{PID: 2, Name: strPtr("python")}

	idx.Index(e1)
	idx.Index(e2)

	bm := idx.GetBitmapForProcessName("Chrome")
	require.NotNil(t, bm)
	assert.Equal(t, uint64(1), bm.GetCardinality())

	assert.Nil(t, idx.GetBitmapForProcessName("Firefox"))
}

func TestBitmapIndexes_PID(t *testing.T) {
	idx := newTestIndexer(nil)

	e1 := makeEntry("e1", "https://a.com/", "GET", 200)
	e1.Process = &client.ProcessInfo{PID: 1234}
	idx.Index(e1)

	bm := idx.GetBitmapForPID(1234)
	require.NotNil(t, bm)
	assert.True(t, bm.Contains(0))

	assert.Nil(t, idx.GetBitmapForPID(9999))
}

func TestBitmapIndexes_HTTPVersion(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://a.com/", "GET", 200))

	bm := idx.GetBitmapForHTTPVersion("h2")
	require.NotNil(t, bm)
	assert.Equal(t, uint64(1), bm.GetCardinality())

	assert.Nil(t, idx.GetBitmapForHTTPVersion("HTTP/1.1"))
}

func TestBitmapIndexes_HeaderName(t *testing.T) {
	idx := newTestIndexer(nil)

	e := makeEntry("e1", "https://a.com/", "GET", 200)
	e.Request.Headers = client.Headers{
		{"Authorization", "Bearer tok"},
		{"Content-Type", "application/json"},
	}
	idx.Index(e)

	bm := idx.GetBitmapForHeaderName("authorization")
	require.NotNil(t, bm)
	assert.True(t, bm.Contains(0))

	bm = idx.GetBitmapForHeaderName("content-type")
	require.NotNil(t, bm)
	assert.True(t, bm.Contains(0))

	assert.Nil(t, idx.GetBitmapForHeaderName("x-custom"))
}

func TestBitmapIndexes_TLS(t *testing.T) {
	idx := newTestIndexer(nil)

	e := makeEntry("e1", "https://a.com/", "GET", 200)
	e.TLS = client.TLSInfo{
		ConnectionID: strPtr("tls-1"),
		JA3:          &client.JA3Fingerprint{Hash: "j3hash"},
		JA4:          &client.JA4Fingerprint{Hashed: "j4hash"},
	}
	idx.Index(e)

	assert.NotNil(t, idx.GetBitmapForTLSConnection("tls-1"))
	assert.Nil(t, idx.GetBitmapForTLSConnection("tls-999"))

	assert.NotNil(t, idx.GetBitmapForJA3("j3hash"))
	assert.Nil(t, idx.GetBitmapForJA3("nope"))

	assert.NotNil(t, idx.GetBitmapForJA4("j4hash"))
	assert.Nil(t, idx.GetBitmapForJA4("nope"))
}

func TestBitmapIndexes_H2Connection(t *testing.T) {
	idx := newTestIndexer(nil)

	e := makeEntry("e1", "https://a.com/", "GET", 200)
	e.HTTP2 = &client.HTTP2Info{ConnectionID: "h2-1", StreamID: 3}
	idx.Index(e)

	assert.NotNil(t, idx.GetBitmapForH2Connection("h2-1"))
	assert.Nil(t, idx.GetBitmapForH2Connection("h2-999"))
}

func TestBitmapIndexes_URLTokens(t *testing.T) {
	idx := newTestIndexer(nil)
	idx.Index(makeEntry("e1", "https://api.example.com/users/search?q=hello", "GET", 200))

	// URL tokens should include host segments, path segments, query values
	assert.NotNil(t, idx.GetBitmapForToken("api"))
	assert.NotNil(t, idx.GetBitmapForToken("example"))
	assert.NotNil(t, idx.GetBitmapForToken("users"))
	assert.NotNil(t, idx.GetBitmapForToken("search"))
	assert.NotNil(t, idx.GetBitmapForToken("hello"))
	assert.Nil(t, idx.GetBitmapForToken("nonexistent"))
}

func TestBitmapIndexes_HeaderTokens(t *testing.T) {
	idx := newTestIndexer(nil)

	e := makeEntry("e1", "https://a.com/", "GET", 200)
	e.Request.Headers = client.Headers{
		{"Authorization", "Bearer my-secret-token"},
	}
	idx.Index(e)

	// Header tokens from "authorization: Bearer my-secret-token"
	assert.NotNil(t, idx.GetBitmapForHeaderToken("authorization"))
	assert.NotNil(t, idx.GetBitmapForHeaderToken("bearer"))
	assert.NotNil(t, idx.GetBitmapForHeaderToken("secret"))
	assert.NotNil(t, idx.GetBitmapForHeaderToken("token"))
	assert.Nil(t, idx.GetBitmapForHeaderToken("nonexistent"))
}

func TestBitmapIndexes_BodyTokens_Disabled(t *testing.T) {
	idx := newTestIndexer(&config.Config{IndexBody: false})

	body := base64.StdEncoding.EncodeToString([]byte(`{"product":"widget"}`))
	e := makeEntry("e1", "https://a.com/", "POST", 200)
	e.Response = &client.Response{
		StatusCode: intPtr(200),
		Headers:    client.Headers{{"Content-Type", "application/json"}},
		Body:       &body,
	}
	idx.Index(e)

	// Body indexing disabled, so no body tokens
	assert.Nil(t, idx.GetBitmapForBodyToken("product"))
	assert.Nil(t, idx.GetBitmapForBodyToken("widget"))
	assert.False(t, idx.BodyIndexEnabled())
}

func TestBitmapIndexes_BodyTokens_Enabled(t *testing.T) {
	idx := newTestIndexer(&config.Config{IndexBody: true, IndexBodyMaxBytes: 65536})

	body := base64.StdEncoding.EncodeToString([]byte(`{"product":"widget","count":5}`))
	e := makeEntry("e1", "https://a.com/", "POST", 200)
	e.Response = &client.Response{
		StatusCode: intPtr(200),
		Headers:    client.Headers{{"Content-Type", "application/json"}},
		Body:       &body,
	}
	idx.Index(e)

	assert.True(t, idx.BodyIndexEnabled())
	assert.NotNil(t, idx.GetBitmapForBodyToken("product"))
	assert.NotNil(t, idx.GetBitmapForBodyToken("widget"))
}

func TestBitmapIndexes_BodyTokens_RequestBody(t *testing.T) {
	idx := newTestIndexer(&config.Config{IndexBody: true, IndexBodyMaxBytes: 65536})

	reqBody := base64.StdEncoding.EncodeToString([]byte(`{"action":"create"}`))
	e := makeEntry("e1", "https://a.com/", "POST", 200)
	e.Request.Headers = append(e.Request.Headers, []string{"content-type", "application/json"})
	e.Request.Body = &reqBody
	idx.Index(e)

	assert.NotNil(t, idx.GetBitmapForBodyToken("action"))
	assert.NotNil(t, idx.GetBitmapForBodyToken("create"))
}

func TestBitmapIndexes_HeaderValue(t *testing.T) {
	idx := newTestIndexer(nil)

	e := makeEntry("e1", "https://a.com/", "GET", 200)
	e.Request.Headers = client.Headers{
		{"Content-Type", "application/json"},
	}
	idx.Index(e)

	bm := idx.GetBitmapForHeaderValue("content-type", "application/json")
	require.NotNil(t, bm)
	assert.True(t, bm.Contains(0))

	assert.Nil(t, idx.GetBitmapForHeaderValue("content-type", "text/html"))
}

func TestBodyIndexEnabled(t *testing.T) {
	assert.False(t, newTestIndexer(nil).BodyIndexEnabled())
	assert.False(t, newTestIndexer(&config.Config{IndexBody: false}).BodyIndexEnabled())
	assert.True(t, newTestIndexer(&config.Config{IndexBody: true}).BodyIndexEnabled())

	// nil config
	idx := &Indexer{}
	assert.False(t, idx.BodyIndexEnabled())
}
