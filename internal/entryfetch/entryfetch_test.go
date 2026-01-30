package entryfetch

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/pkg/client"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func newTestCache() *cache.EntryCache {
	c, _ := cache.NewEntryCache(128)
	return c
}

func makeEntry(id string) *client.SessionEntry {
	body := base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))
	return &client.SessionEntry{
		ID:          id,
		URL:         "https://example.com/api",
		HTTPVersion: "h2",
		Request: client.Request{
			Method: strPtr("GET"),
			Headers: client.Headers{
				{"Host", "example.com"},
				{"Content-Type", "application/x-www-form-urlencoded"},
			},
			Body: &body,
		},
		Response: &client.Response{
			StatusCode: intPtr(200),
			Headers: client.Headers{
				{"Content-Type", "application/json"},
			},
			Body: &body,
		},
	}
}

func TestFetchEntry_CacheHit(t *testing.T) {
	ec := newTestCache()
	entry := makeEntry("e1")
	ec.Put("e1", entry)

	// Client is nil -- should never be called on cache hit
	got, err := FetchEntry(context.Background(), nil, ec, "active", "e1")
	require.NoError(t, err)
	assert.Equal(t, entry, got)
}

func TestDecodeBody_Response(t *testing.T) {
	entry := makeEntry("e1")

	body, ct, err := DecodeBody(entry, "response")
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Equal(t, []byte(`{"key":"value"}`), body)
}

func TestDecodeBody_Request(t *testing.T) {
	entry := makeEntry("e1")

	body, ct, err := DecodeBody(entry, "request")
	require.NoError(t, err)
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	assert.Equal(t, []byte(`{"key":"value"}`), body)
}

func TestDecodeBody_NoResponse(t *testing.T) {
	entry := makeEntry("e1")
	entry.Response = nil

	body, ct, err := DecodeBody(entry, "response")
	require.NoError(t, err)
	assert.Nil(t, body)
	assert.Empty(t, ct)
}

func TestDecodeBody_NilBody(t *testing.T) {
	entry := makeEntry("e1")
	entry.Response.Body = nil

	body, ct, err := DecodeBody(entry, "response")
	require.NoError(t, err)
	assert.Nil(t, body)
	assert.Equal(t, "application/json", ct)
}

func TestDecodeBody_EmptyBody(t *testing.T) {
	entry := makeEntry("e1")
	empty := ""
	entry.Response.Body = &empty

	body, ct, err := DecodeBody(entry, "response")
	require.NoError(t, err)
	assert.Nil(t, body)
	assert.Equal(t, "application/json", ct)
}

func TestDecodeBody_InvalidBase64(t *testing.T) {
	entry := makeEntry("e1")
	invalid := "not-valid-base64!!!"
	entry.Response.Body = &invalid

	body, ct, err := DecodeBody(entry, "response")
	assert.Error(t, err)
	assert.Nil(t, body)
	assert.Equal(t, "application/json", ct)
}
