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
			name:     "full URL with host path query",
			input:    "https://api.example.com/v1/users?id=123&name=test",
			expected: []string{"api", "example", "com", "v1", "users", "id", "name"},
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
			name:     "query keys extracted not values",
			input:    "https://example.com/search?query=test&limit=10",
			expected: []string{"example", "com", "search", "query", "limit"},
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
			assert.Equal(t, tt.expected, result, "TokenizeURL(%q)", tt.input)
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
