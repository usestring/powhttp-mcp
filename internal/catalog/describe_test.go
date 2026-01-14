package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

func TestAnalyzeHeaders(t *testing.T) {
	tests := []struct {
		name        string
		entries     []*client.SessionEntry
		expectedLen int
		expectedTop string
	}{
		{
			name:        "empty entries",
			entries:     []*client.SessionEntry{},
			expectedLen: 0,
		},
		{
			name: "single entry",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
							{"Accept", "*/*"},
						},
					},
				},
			},
			expectedLen: 2,
			expectedTop: "content-type",
		},
		{
			name: "multiple entries same headers",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
							{"Accept", "*/*"},
						},
					},
				},
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
							{"Accept", "*/*"},
						},
					},
				},
			},
			expectedLen: 2,
		},
		{
			name: "varying headers",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
							{"Authorization", "Bearer token"},
						},
					},
				},
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
						},
					},
				},
			},
			expectedLen: 2,
		},
		{
			name: "case insensitive",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Content-Type", "application/json"},
						},
					},
				},
				{
					Request: client.Request{
						Headers: client.Headers{
							{"content-type", "text/html"},
						},
					},
				},
			},
			expectedLen: 1,
			expectedTop: "content-type",
		},
		{
			name: "limit to top 20",
			entries: func() []*client.SessionEntry {
				headers := make(client.Headers, 25)
				for i := 0; i < 25; i++ {
					headers[i] = []string{string(rune('a' + i)), "value"}
				}
				return []*client.SessionEntry{{Request: client.Request{Headers: headers}}}
			}(),
			expectedLen: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeHeaders(tt.entries)

			assert.Len(t, result, tt.expectedLen)

			if tt.expectedTop != "" && len(result) > 0 {
				assert.Equal(t, tt.expectedTop, result[0].Name)
			}

			// Verify frequencies are between 0 and 1
			for _, freq := range result {
				assert.GreaterOrEqual(t, freq.Frequency, 0.0)
				assert.LessOrEqual(t, freq.Frequency, 1.0)
			}

			// Verify sorted by frequency descending
			for i := 1; i < len(result); i++ {
				assert.GreaterOrEqual(t, result[i-1].Frequency, result[i].Frequency,
					"headers should be sorted by frequency descending")
			}
		})
	}
}

func TestDetectAuthSignals(t *testing.T) {
	tests := []struct {
		name           string
		entries        []*client.SessionEntry
		wantCookies    bool
		wantBearer     bool
		wantCustomAuth []string
	}{
		{
			name:    "no auth",
			entries: []*client.SessionEntry{},
		},
		{
			name: "bearer token",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
						},
					},
				},
			},
			wantBearer: true,
		},
		{
			name: "cookies present",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Cookie", "session_id=abc123"},
						},
					},
				},
			},
			wantCookies: true,
		},
		{
			name: "custom auth headers",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"X-API-Key", "secret123"},
						},
					},
				},
			},
			wantCustomAuth: []string{"x-api-key"},
		},
		{
			name: "multiple custom auth headers",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"X-API-Key", "secret123"},
							{"X-Auth-Token", "token456"},
						},
					},
				},
			},
			wantCustomAuth: []string{"x-api-key", "x-auth-token"},
		},
		{
			name: "all auth types",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Cookie", "session=abc"},
							{"Authorization", "Bearer token"},
							{"X-API-Key", "key123"},
						},
					},
				},
			},
			wantCookies:    true,
			wantBearer:     true,
			wantCustomAuth: []string{"x-api-key"},
		},
		{
			name: "basic auth not detected as bearer",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Authorization", "Basic dXNlcjpwYXNz"},
						},
					},
				},
			},
			wantBearer: false,
		},
		{
			name: "case insensitive bearer",
			entries: []*client.SessionEntry{
				{
					Request: client.Request{
						Headers: client.Headers{
							{"Authorization", "BEARER token"},
						},
					},
				},
			},
			wantBearer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectAuthSignals(tt.entries)

			assert.Equal(t, tt.wantCookies, result.CookiesPresent, "CookiesPresent")
			assert.Equal(t, tt.wantBearer, result.BearerPresent, "BearerPresent")
			assert.ElementsMatch(t, tt.wantCustomAuth, result.CustomAuthHeaders, "CustomAuthHeaders")
		})
	}
}

func TestAnalyzeQueryKeys(t *testing.T) {
	tests := []struct {
		name         string
		entries      []*client.SessionEntry
		wantStable   []string
		wantVolatile []string
	}{
		{
			name:    "empty entries",
			entries: []*client.SessionEntry{},
		},
		{
			name: "stable and volatile keys",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/users?page=1&limit=10"},
				{URL: "https://api.example.com/users?page=2&limit=10"},
				{URL: "https://api.example.com/users?page=3&limit=10"},
			},
			wantStable:   []string{"limit"},
			wantVolatile: []string{"page"}, // page has all unique values so it's volatile
		},
		{
			name: "volatile keys by name",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?timestamp=1000"},
				{URL: "https://api.example.com/data?timestamp=2000"},
			},
			wantVolatile: []string{"timestamp"},
		},
		{
			name: "volatile keys by uniqueness",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?id=abc"},
				{URL: "https://api.example.com/data?id=def"},
				{URL: "https://api.example.com/data?id=ghi"},
			},
			wantVolatile: []string{"id"},
		},
		{
			name: "mixed stable and volatile",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?api_key=secret&timestamp=1000"},
				{URL: "https://api.example.com/data?api_key=secret&timestamp=2000"},
				{URL: "https://api.example.com/data?api_key=secret&timestamp=3000"},
			},
			wantStable:   []string{"api_key"},
			wantVolatile: []string{"timestamp"},
		},
		{
			name: "underscore key detected as volatile",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?_=1000"},
				{URL: "https://api.example.com/data?_=2000"},
			},
			wantVolatile: []string{"_"},
		},
		{
			name: "nonce detected as volatile",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?nonce=abc123"},
				{URL: "https://api.example.com/data?nonce=def456"},
			},
			wantVolatile: []string{"nonce"},
		},
		{
			name: "infrequent key marked volatile if unique",
			entries: []*client.SessionEntry{
				{URL: "https://api.example.com/data?key=value"},
				{URL: "https://api.example.com/data"},
				{URL: "https://api.example.com/data"},
			},
			wantStable:   []string{},
			wantVolatile: []string{"key"}, // appears only once but has unique value, so volatile
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeQueryKeys(tt.entries)

			assert.ElementsMatch(t, tt.wantStable, result.Stable, "Stable keys")
			assert.ElementsMatch(t, tt.wantVolatile, result.Volatile, "Volatile keys")

			// Verify sorted
			assert.IsIncreasing(t, result.Stable, "Stable keys should be sorted")
			assert.IsIncreasing(t, result.Volatile, "Volatile keys should be sorted")
		})
	}
}

func TestAnalyzeQueryKeys_EdgeCases(t *testing.T) {
	t.Run("invalid URL handled", func(t *testing.T) {
		entries := []*client.SessionEntry{
			{URL: "://invalid"},
		}
		result := analyzeQueryKeys(entries)
		assert.Empty(t, result.Stable)
		assert.Empty(t, result.Volatile)
	})

	t.Run("empty query string", func(t *testing.T) {
		entries := []*client.SessionEntry{
			{URL: "https://api.example.com/data"},
		}
		result := analyzeQueryKeys(entries)
		assert.Empty(t, result.Stable)
		assert.Empty(t, result.Volatile)
	})

	t.Run("multiple values for same key", func(t *testing.T) {
		entries := []*client.SessionEntry{
			{URL: "https://api.example.com/data?tag=a&tag=b"},
			{URL: "https://api.example.com/data?tag=c&tag=d"},
		}
		result := analyzeQueryKeys(entries)
		// tag appears in all entries, should be analyzed
		assert.True(t, contains(result.Stable, "tag") || contains(result.Volatile, "tag"),
			"tag should be analyzed")
	})
}

func TestAnalyzeHeaders_Deduplication(t *testing.T) {
	entries := []*client.SessionEntry{
		{
			Request: client.Request{
				Headers: client.Headers{
					{"Content-Type", "application/json"},
					{"Content-Type", "charset=utf-8"}, // Duplicate header in same request
				},
			},
		},
	}

	result := analyzeHeaders(entries)

	require.Len(t, result, 1, "should only count content-type once per request")
	assert.Equal(t, 1.0, result[0].Frequency, "frequency should be 1.0")
}

func TestDetectAuthSignals_Deduplication(t *testing.T) {
	entries := []*client.SessionEntry{
		{
			Request: client.Request{
				Headers: client.Headers{
					{"X-API-Key", "key1"},
					{"X-API-Key", "key2"}, // Duplicate in same request
				},
			},
		},
		{
			Request: client.Request{
				Headers: client.Headers{
					{"X-API-Key", "key3"},
				},
			},
		},
	}

	result := detectAuthSignals(entries)

	// Count x-api-key occurrences
	count := 0
	for _, header := range result.CustomAuthHeaders {
		if header == "x-api-key" {
			count++
		}
	}

	assert.Equal(t, 1, count, "x-api-key should appear once")
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
