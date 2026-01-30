package indexer

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestFromSessionEntry_Basic(t *testing.T) {
	entry := &client.SessionEntry{
		ID:          "entry-1",
		URL:         "https://api.example.com/users/123?page=1",
		HTTPVersion: "h2",
		Request: client.Request{
			Method: strPtr("GET"),
			Headers: client.Headers{
				{"Authorization", "Bearer tok123"},
				{"Content-Type", "application/json"},
			},
		},
		Response: &client.Response{
			StatusCode: intPtr(200),
			Headers: client.Headers{
				{"Content-Type", "application/json; charset=utf-8"},
				{"Set-Cookie", "session_id=abc; Path=/; HttpOnly"},
			},
		},
		Timings: client.Timings{
			StartedAt: 1700000000000,
		},
		Process: &client.ProcessInfo{
			PID:  1234,
			Name: strPtr("Chrome"),
		},
		TLS: client.TLSInfo{
			ConnectionID: strPtr("tls-conn-1"),
			JA3:          &client.JA3Fingerprint{Hash: "ja3hash"},
			JA4:          &client.JA4Fingerprint{Hashed: "ja4hash"},
		},
		HTTP2: &client.HTTP2Info{
			ConnectionID: "h2-conn-1",
			StreamID:     5,
		},
	}

	meta := FromSessionEntry(entry)

	assert.Equal(t, "entry-1", meta.EntryID)
	assert.Equal(t, int64(1700000000000), meta.TsMs)
	assert.Equal(t, "GET", meta.Method)
	assert.Equal(t, "https://api.example.com/users/123?page=1", meta.URL)
	assert.Equal(t, "api.example.com", meta.Host)
	assert.Equal(t, "/users/123", meta.Path)
	assert.Equal(t, "h2", meta.HTTPVersion)
	assert.Equal(t, 200, meta.Status)
	assert.Equal(t, "Chrome", meta.ProcessName)
	assert.Equal(t, 1234, meta.PID)
	assert.Equal(t, "tls-conn-1", meta.TLSConnectionID)
	assert.Equal(t, "ja3hash", meta.JA3)
	assert.Equal(t, "ja4hash", meta.JA4)
	assert.Equal(t, "h2-conn-1", meta.H2ConnectionID)
	assert.Equal(t, 5, meta.H2StreamID)
	assert.Equal(t, "application/json", meta.RespContentType)

	// Auth fields
	assert.Equal(t, "Bearer tok123", meta.AuthHeader)
	require.NotNil(t, meta.SetCookies)
	assert.Equal(t, "abc", meta.SetCookies["session_id"])
}

func TestFromSessionEntry_NilFields(t *testing.T) {
	entry := &client.SessionEntry{
		ID:  "entry-2",
		URL: "http://example.com/path",
		Request: client.Request{
			Method: nil, // nil method
		},
		Timings: client.Timings{StartedAt: 1000},
	}

	meta := FromSessionEntry(entry)

	assert.Equal(t, "entry-2", meta.EntryID)
	assert.Equal(t, "", meta.Method)
	assert.Equal(t, 0, meta.Status)
	assert.Equal(t, "", meta.ProcessName)
	assert.Equal(t, 0, meta.PID)
	assert.Equal(t, "", meta.TLSConnectionID)
	assert.Equal(t, "", meta.JA3)
	assert.Equal(t, "", meta.JA4)
	assert.Equal(t, "", meta.H2ConnectionID)
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"full URL", "https://API.Example.COM/path", "api.example.com"},
		{"with port", "http://localhost:8080/foo", "localhost:8080"},
		{"empty", "", ""},
		{"invalid", "://bad", ""},
		{"relative path", "/api/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractHost(tt.url))
		})
	}
}

func TestExtractPath(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"full URL", "https://example.com/api/users?q=1", "/api/users"},
		{"root", "https://example.com/", "/"},
		{"no path", "https://example.com", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractPath(tt.url))
		})
	}
}

func TestExtractHeaderNames(t *testing.T) {
	tests := []struct {
		name     string
		headers  client.Headers
		expected []string
	}{
		{
			"basic",
			client.Headers{{"Content-Type", "text/html"}, {"Authorization", "Bearer x"}},
			[]string{"content-type", "authorization"},
		},
		{
			"deduplicates",
			client.Headers{{"Host", "a"}, {"host", "b"}},
			[]string{"host"},
		},
		{
			"empty",
			client.Headers{},
			[]string{},
		},
		{
			"nil",
			nil,
			[]string{},
		},
		{
			"single element pair",
			client.Headers{{"Content-Type"}},
			[]string{"content-type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHeaderNames(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractHeaderValues(t *testing.T) {
	headers := client.Headers{
		{"Content-Type", "application/json"},
		{"Authorization", "Bearer tok"},
		{"X-Only-Name"},
	}

	result := extractHeaderValues(headers)

	assert.Len(t, result, 2)
	assert.Equal(t, "content-type", result[0].Name)
	assert.Equal(t, "application/json", result[0].Value)
	assert.Equal(t, "authorization", result[1].Name)
	assert.Equal(t, "Bearer tok", result[1].Value)
}

func TestIsSessionCookie(t *testing.T) {
	tests := []struct {
		name     string
		cookie   string
		expected bool
	}{
		{"JSESSIONID", "JSESSIONID", true},
		{"PHPSESSID", "PHPSESSID", true},
		{"connect.sid", "connect.sid", true},
		{"custom session", "my_session_id", true},
		{"auth token", "auth_token", true},
		{"jwt cookie", "jwt_data", true},
		{"regular cookie", "theme", false},
		{"tracking", "_ga", false},
		{"preference", "lang", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSessionCookie(tt.cookie))
		})
	}
}

func TestExtractSessionCookies(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected map[string]string
	}{
		{
			"session cookies",
			"session_id=abc123; theme=dark; jwt_token=xyz",
			map[string]string{"session_id": "abc123", "jwt_token": "xyz"},
		},
		{
			"empty",
			"",
			nil,
		},
		{
			"no session cookies",
			"theme=dark; lang=en",
			nil,
		},
		{
			"PHPSESSID",
			"PHPSESSID=abc123",
			map[string]string{"PHPSESSID": "abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSessionCookies(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		headers  client.Headers
		expected map[string]string
	}{
		{
			"x-api-key",
			client.Headers{{"X-Api-Key", "key123"}, {"Content-Type", "json"}},
			map[string]string{"x-api-key": "key123"},
		},
		{
			"multiple auth headers",
			client.Headers{{"X-Api-Key", "k1"}, {"X-Auth-Token", "t1"}},
			map[string]string{"x-api-key": "k1", "x-auth-token": "t1"},
		},
		{
			"no auth headers",
			client.Headers{{"Content-Type", "json"}},
			nil,
		},
		{
			"empty",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAPIKeys(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSetCookies(t *testing.T) {
	tests := []struct {
		name     string
		headers  client.Headers
		expected map[string]string
	}{
		{
			"session cookie with attributes",
			client.Headers{
				{"Set-Cookie", "session_id=abc; Path=/; HttpOnly"},
			},
			map[string]string{"session_id": "abc"},
		},
		{
			"session cookie without attributes",
			client.Headers{
				{"Set-Cookie", "auth_token=xyz"},
			},
			map[string]string{"auth_token": "xyz"},
		},
		{
			"non-session cookie",
			client.Headers{
				{"Set-Cookie", "theme=dark; Path=/"},
			},
			nil,
		},
		{
			"mixed",
			client.Headers{
				{"Set-Cookie", "session_id=a; Path=/"},
				{"Set-Cookie", "theme=dark"},
				{"Content-Type", "text/html"},
			},
			map[string]string{"session_id": "a"},
		},
		{
			"empty",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSetCookies(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeBodySize(t *testing.T) {
	tests := []struct {
		name     string
		encoded  *string
		expected int
	}{
		{"nil", nil, 0},
		{"empty", strPtr(""), 0},
		{"hello", strPtr(base64.StdEncoding.EncodeToString([]byte("hello"))), 5},
		{"json body", strPtr(base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))), 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, computeBodySize(tt.encoded))
		})
	}
}

func TestNormalizeContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "application/json", "application/json"},
		{"with charset", "application/json; charset=utf-8", "application/json"},
		{"with boundary", "multipart/form-data; boundary=----", "multipart/form-data"},
		{"uppercase", "Application/JSON", "application/json"},
		{"spaces", " text/html ; charset=utf-8", "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeContentType(tt.input))
		})
	}
}

func TestToSummary(t *testing.T) {
	meta := &EntryMeta{
		EntryID:     "e1",
		TsMs:        1234567890,
		Method:      "POST",
		URL:         "https://example.com/api",
		Host:        "example.com",
		Path:        "/api",
		Status:      201,
		HTTPVersion: "h2",
		ProcessName: "curl",
		PID:         42,
		TLSConnectionID: "tls-1",
		JA3:             "j3",
		JA4:             "j4",
		H2ConnectionID:  "h2-1",
		H2StreamID:      3,
		ReqBodyBytes:    100,
		RespBodyBytes:   500,
		RespContentType: "application/json",
	}

	summary := meta.ToSummary()

	assert.Equal(t, "e1", summary.EntryID)
	assert.Equal(t, int64(1234567890), summary.TsMs)
	assert.Equal(t, "POST", summary.Method)
	assert.Equal(t, "https://example.com/api", summary.URL)
	assert.Equal(t, "example.com", summary.Host)
	assert.Equal(t, "/api", summary.Path)
	assert.Equal(t, 201, summary.Status)
	assert.Equal(t, "h2", summary.HTTPVersion)
	assert.Equal(t, "curl", summary.ProcessName)
	assert.Equal(t, 42, summary.PID)
	assert.Equal(t, "tls-1", summary.TLS.ConnectionID)
	assert.Equal(t, "j3", summary.TLS.JA3)
	assert.Equal(t, "j4", summary.TLS.JA4)
	assert.Equal(t, "h2-1", summary.HTTP2.ConnectionID)
	assert.Equal(t, 3, summary.HTTP2.StreamID)
	assert.Equal(t, 100, summary.Sizes.ReqBodyBytes)
	assert.Equal(t, 500, summary.Sizes.RespBodyBytes)
	assert.Equal(t, "application/json", summary.Sizes.RespContentType)
}
