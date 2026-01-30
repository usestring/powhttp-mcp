package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple path",
			input:    "/api/users",
			expected: []string{"api", "users"},
		},
		{
			name:     "path with query",
			input:    "/api/users?id=123&name=test",
			expected: []string{"api", "users", "id", "123", "name", "test"},
		},
		{
			name:     "URL with dots and hyphens",
			input:    "api.example-site.com/v1/users",
			expected: []string{"api", "example", "site", "com", "v1", "users"},
		},
		{
			name:     "mixed case lowercased",
			input:    "API/Users/ENDPOINT",
			expected: []string{"api", "users", "endpoint"},
		},
		{
			name:     "tokens with underscores",
			input:    "api_v1/user_id",
			expected: []string{"api", "v1", "user", "id"},
		},
		{
			name:     "filter short tokens",
			input:    "a/b/cd/def",
			expected: []string{"cd", "def"},
		},
		{
			name:     "colons as delimiters",
			input:    "user:123:profile",
			expected: []string{"user", "123", "profile"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only delimiters",
			input:    "/?&=.-_:",
			expected: []string{},
		},
		{
			name:     "single char tokens filtered",
			input:    "a/b/c",
			expected: []string{},
		},
		{
			name:     "spaces as delimiters",
			input:    "hello world test",
			expected: []string{"hello", "world", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tokenize(tt.input)
			assert.Equal(t, tt.expected, result, "Tokenize(%q) returned unexpected result", tt.input)
		})
	}
}

func TestNormalizePathSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"numeric segment", "123", "{id}"},
		{"large numeric", "999999999", "{id}"},
		{"UUID lowercase", "550e8400-e29b-41d4-a716-446655440000", "{uuid}"},
		{"UUID uppercase", "550E8400-E29B-41D4-A716-446655440000", "{uuid}"},
		{"UUID mixed case", "550e8400-E29B-41d4-A716-446655440000", "{uuid}"},
		{"hex string 8 chars", "deadbeef", "{hex}"},
		{"hex string longer", "deadbeef12345678", "{hex}"},
		{"hex uppercase converted", "DEADBEEF", "{hex}"},
		{"hex too short", "dead", "dead"},
		{"alphanumeric not normalized", "user123", "user123"},
		{"plain text segment", "users", "users"},
		{"segment with special chars", "user-profile", "user-profile"},
		{"hex with non-hex chars", "deadbeefghij", "deadbeefghij"},
		{"zero", "0", "{id}"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePathSegment(tt.input)
			assert.Equal(t, tt.expected, result, "NormalizePathSegment(%q)", tt.input)
		})
	}
}

func TestTokenizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "full URL with host path query keys and values",
			input:    "https://api.example.com/v1/users?id=123&name=test",
			expected: []string{"api", "example", "com", "v1", "users", "id", "123", "name", "test"},
		},
		{
			name:     "URL with port",
			input:    "http://localhost:8080/api/endpoint",
			expected: []string{"localhost", "8080", "api", "endpoint"},
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/page#section",
			expected: []string{"example", "com", "page"},
		},
		{
			name:     "URL without query",
			input:    "https://api.example.com/users",
			expected: []string{"api", "example", "com", "users"},
		},
		{
			name:     "relative path",
			input:    "/api/users",
			expected: []string{"api", "users"},
		},
		{
			name:     "query keys and values extracted",
			input:    "https://example.com/search?query=test&limit=10",
			expected: []string{"example", "com", "search", "query", "test", "limit", "10"},
		},
		{
			name:     "invalid URL falls back to tokenize",
			input:    "not a valid URL ://",
			expected: []string{"not", "valid", "url"},
		},
		{
			name:     "empty URL",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenizeURL(tt.input)
			assert.ElementsMatch(t, tt.expected, result, "TokenizeURL(%q)", tt.input)
		})
	}
}

func TestTokenizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple path", "/api/users", []string{"api", "users"}},
		{"path with IDs", "/api/users/123", []string{"api", "users", "123"}},
		{"nested path", "/v1/api/users/posts", []string{"v1", "api", "users", "posts"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenizePath(tt.input)
			assert.Equal(t, tt.expected, result, "TokenizePath(%q)", tt.input)
		})
	}
}

func TestTokenizeHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  []HeaderValue
		expected []string
	}{
		{
			name:     "nil headers",
			headers:  nil,
			expected: nil,
		},
		{
			name:     "empty headers",
			headers:  []HeaderValue{},
			expected: nil,
		},
		{
			name: "authorization bearer",
			headers: []HeaderValue{
				{Name: "authorization", Value: "Bearer eyJhbGciOiJIUzI1NiJ9"},
			},
			expected: []string{"authorization", "bearer", "eyjhbgcioijiuzi1nij9"},
		},
		{
			name: "user-agent with semicolons",
			headers: []HeaderValue{
				{Name: "user-agent", Value: "Mozilla/5.0; Windows; x64"},
			},
			expected: []string{"user", "agent", "mozilla", "windows", "x64"},
		},
		{
			name: "content-type json",
			headers: []HeaderValue{
				{Name: "content-type", Value: "application/json; charset=utf-8"},
			},
			expected: []string{"content", "type", "application", "json", "charset", "utf"},
		},
		{
			name: "multiple headers",
			headers: []HeaderValue{
				{Name: "host", Value: "api.example.com"},
				{Name: "accept", Value: "text/html"},
			},
			expected: []string{"host", "api", "example", "com", "accept", "text", "html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenizeHeaders(tt.headers)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result, "TokenizeHeaders")
			}
		})
	}
}

func TestTokenizeBody(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		maxBytes    int
		expected    []string
	}{
		{
			name:        "empty body",
			contentType: "application/json",
			body:        nil,
			maxBytes:    65536,
			expected:    nil,
		},
		{
			name:        "json object with keys and string values",
			contentType: "application/json",
			body:        []byte(`{"name":"John","age":30,"city":"New York"}`),
			maxBytes:    65536,
			expected:    []string{"name", "john", "age", "city", "new", "york"},
		},
		{
			name:        "json nested object",
			contentType: "application/json",
			body:        []byte(`{"user":{"email":"test@example.com","role":"admin"}}`),
			maxBytes:    65536,
			expected:    []string{"user", "email", "test", "example", "com", "role", "admin"},
		},
		{
			name:        "json array",
			contentType: "application/json",
			body:        []byte(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`),
			maxBytes:    65536,
			expected:    []string{"id", "name", "alice", "name", "bob", "id"},
		},
		{
			name:        "html strips tags",
			contentType: "text/html",
			body:        []byte(`<html><body><h1>Hello World</h1><p>Test content</p></body></html>`),
			maxBytes:    65536,
			expected:    []string{"hello", "world", "test", "content"},
		},
		{
			name:        "xml strips tags",
			contentType: "application/xml",
			body:        []byte(`<root><item>value one</item><item>value two</item></root>`),
			maxBytes:    65536,
			expected:    []string{"value", "one", "value", "two"},
		},
		{
			name:        "plain text",
			contentType: "text/plain",
			body:        []byte("hello world test content"),
			maxBytes:    65536,
			expected:    []string{"hello", "world", "test", "content"},
		},
		{
			name:        "form-encoded",
			contentType: "application/x-www-form-urlencoded",
			body:        []byte("username=admin&password=secret123"),
			maxBytes:    65536,
			expected:    []string{"username", "admin", "password", "secret123"},
		},
		{
			name:        "binary content type skipped",
			contentType: "image/png",
			body:        []byte{0x89, 0x50, 0x4e, 0x47},
			maxBytes:    65536,
			expected:    nil,
		},
		{
			name:        "octet-stream skipped",
			contentType: "application/octet-stream",
			body:        []byte("some binary data"),
			maxBytes:    65536,
			expected:    nil,
		},
		{
			name:        "max bytes truncation",
			contentType: "text/plain",
			body:        []byte("hello world this is a longer text that should be truncated"),
			maxBytes:    11,
			expected:    []string{"hello", "world"},
		},
		{
			name:        "json with charset in content type",
			contentType: "application/json; charset=utf-8",
			body:        []byte(`{"key":"value"}`),
			maxBytes:    65536,
			expected:    []string{"key", "value"},
		},
		{
			name:        "csv as text",
			contentType: "text/csv",
			body:        []byte("name,email\nAlice,alice@example.com"),
			maxBytes:    65536,
			expected:    []string{"name", "email", "alice", "alice", "example", "com"},
		},
		{
			name:        "empty content type skipped",
			contentType: "",
			body:        []byte("some data"),
			maxBytes:    65536,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TokenizeBody(tt.contentType, tt.body, tt.maxBytes)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result, "TokenizeBody")
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"path with numeric ID", "/users/123/posts", "/users/{id}/posts"},
		{"path with UUID", "/users/550e8400-e29b-41d4-a716-446655440000/profile", "/users/{uuid}/profile"},
		{"path with hex", "/objects/deadbeef12345678/details", "/objects/{hex}/details"},
		{"path with multiple IDs", "/users/123/posts/456", "/users/{id}/posts/{id}"},
		{"path with mixed patterns", "/users/550e8400-e29b-41d4-a716-446655440000/items/789", "/users/{uuid}/items/{id}"},
		{"plain path unchanged", "/api/users/posts", "/api/users/posts"},
		{"root path", "/", "/"},
		{"empty path", "", ""},
		{"path with trailing slash", "/users/123/", "/users/{id}/"},
		{"path with alphanumeric not normalized", "/api/v1/user123/posts", "/api/v1/user123/posts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			assert.Equal(t, tt.expected, result, "NormalizePath(%q)", tt.input)
		})
	}
}
