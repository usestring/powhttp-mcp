package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestApplyOptionsDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    *types.ClusterOptions
		expected *types.ClusterOptions
	}{
		{
			name:  "nil input returns defaults",
			input: nil,
			expected: &types.ClusterOptions{
				NormalizeIDs:           true,
				StripVolatileQueryKeys: true,
				ExamplesPerCluster:     3,
				MaxClusters:            200,
			},
		},
		{
			name: "empty input uses defaults",
			input: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				ExamplesPerCluster:     0,
				MaxClusters:            0,
			},
			expected: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				ExamplesPerCluster:     3,
				MaxClusters:            200,
			},
		},
		{
			name: "custom values preserved",
			input: &types.ClusterOptions{
				NormalizeIDs:           true,
				StripVolatileQueryKeys: true,
				ExamplesPerCluster:     5,
				MaxClusters:            100,
			},
			expected: &types.ClusterOptions{
				NormalizeIDs:           true,
				StripVolatileQueryKeys: true,
				ExamplesPerCluster:     5,
				MaxClusters:            100,
			},
		},
		{
			name: "max examples capped at 10",
			input: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				ExamplesPerCluster:     100,
			},
			expected: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				ExamplesPerCluster:     10,
				MaxClusters:            200,
			},
		},
		{
			name: "max clusters capped at 2000",
			input: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				MaxClusters:            5000,
			},
			expected: &types.ClusterOptions{
				NormalizeIDs:           false,
				StripVolatileQueryKeys: false,
				ExamplesPerCluster:     3,
				MaxClusters:            2000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyOptionsDefaults(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPathTemplate(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		normalizeIDs bool
		expected     string
	}{
		{"simple path no normalization", "/api/users", false, "/api/users"},
		{"path with numeric ID normalized", "/api/users/123", true, "/api/users/{id}"},
		{"path with numeric ID not normalized", "/api/users/123", false, "/api/users/123"},
		{"path with UUID normalized", "/api/users/550e8400-e29b-41d4-a716-446655440000/profile", true, "/api/users/{uuid}/profile"},
		{"path with query string stripped", "/api/users?page=1&limit=10", true, "/api/users"},
		{"path with query and ID", "/api/users/123?include=profile", true, "/api/users/{id}"},
		{"path with hex normalized", "/api/objects/deadbeef12345678", true, "/api/objects/{hex}"},
		{"root path", "/", true, "/"},
		{"empty path", "", true, ""},
		{"query only", "?foo=bar", true, ""},
		{"multiple IDs normalized", "/api/users/123/posts/456", true, "/api/users/{id}/posts/{id}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPathTemplate(tt.path, tt.normalizeIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSelectExamples(t *testing.T) {
	tests := []struct {
		name        string
		entryIDs    []string
		count       int
		expectedLen int
	}{
		{"fewer entries than count", []string{"e1", "e2"}, 5, 2},
		{"exact count", []string{"e1", "e2", "e3"}, 3, 3},
		{"select subset", []string{"e1", "e2", "e3", "e4", "e5", "e6"}, 3, 3},
		{"select from large set", []string{"e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8", "e9", "e10"}, 5, 5},
		{"empty input", []string{}, 5, 0},
		{"single entry", []string{"e1"}, 5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectExamples(tt.entryIDs, tt.count)

			assert.Len(t, result, tt.expectedLen)

			// Verify all returned entries exist in input
			entrySet := make(map[string]bool)
			for _, id := range tt.entryIDs {
				entrySet[id] = true
			}

			for _, id := range result {
				assert.True(t, entrySet[id], "returned entry %q not in input", id)
			}

			// Verify spread: first entry should always be included if count > 0
			if len(tt.entryIDs) > 0 && tt.count > 0 && len(result) > 0 {
				assert.Equal(t, tt.entryIDs[0], result[0], "first entry should be included for spread")
			}
		})
	}
}

func TestComputeClusterID(t *testing.T) {
	tests := []struct {
		name string
		key  types.ClusterKey
	}{
		{
			name: "basic cluster",
			key: types.ClusterKey{
				Host:         "api.example.com",
				Method:       "GET",
				PathTemplate: "/users/{id}",
			},
		},
		{
			name: "different host",
			key: types.ClusterKey{
				Host:         "api2.example.com",
				Method:       "GET",
				PathTemplate: "/users/{id}",
			},
		},
		{
			name: "different method",
			key: types.ClusterKey{
				Host:         "api.example.com",
				Method:       "POST",
				PathTemplate: "/users/{id}",
			},
		},
		{
			name: "different path",
			key: types.ClusterKey{
				Host:         "api.example.com",
				Method:       "GET",
				PathTemplate: "/posts/{id}",
			},
		},
	}

	// Test determinism
	for _, tt := range tests {
		t.Run(tt.name+" determinism", func(t *testing.T) {
			id1 := computeClusterID(tt.key)
			id2 := computeClusterID(tt.key)

			assert.Equal(t, id1, id2, "should be deterministic")
			assert.Len(t, id1, 12, "should be 12 characters")
		})
	}

	// Test uniqueness
	t.Run("uniqueness", func(t *testing.T) {
		ids := make(map[string]types.ClusterKey)

		for _, tt := range tests {
			id := computeClusterID(tt.key)

			if existingKey, exists := ids[id]; exists {
				// This would be a collision
				assert.Equal(t, existingKey, tt.key, "collision detected: same ID for different keys")
			}

			ids[id] = tt.key
		}

		assert.Len(t, ids, len(tests), "should have unique IDs for all test cases")
	})
}

func TestComputeScopeHash(t *testing.T) {
	t.Run("nil scope returns default", func(t *testing.T) {
		result := computeScopeHash(nil)
		assert.Equal(t, "default", result)
	})

	t.Run("non-nil scopes generate hex hash", func(t *testing.T) {
		tests := []struct {
			name  string
			scope *types.ClusterScope
		}{
			{"empty scope", &types.ClusterScope{}},
			{"scope with host", &types.ClusterScope{Host: "api.example.com"}},
			{"scope with process", &types.ClusterScope{ProcessName: "chrome", PID: 12345}},
			{"scope with time window", &types.ClusterScope{TimeWindowMs: 60000}},
			{"scope with time range", &types.ClusterScope{SinceMs: 1000000, UntilMs: 2000000}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := computeScopeHash(tt.scope)
				assert.NotEqual(t, "default", result, "should not return default for non-nil scope")
				assert.Len(t, result, 16, "should be 16 characters")
			})
		}
	})

	t.Run("determinism", func(t *testing.T) {
		scope := &types.ClusterScope{
			Host:         "api.example.com",
			ProcessName:  "chrome",
			PID:          12345,
			TimeWindowMs: 60000,
		}

		hash1 := computeScopeHash(scope)
		hash2 := computeScopeHash(scope)

		assert.Equal(t, hash1, hash2, "should be deterministic")
	})

	t.Run("uniqueness", func(t *testing.T) {
		scope1 := &types.ClusterScope{Host: "api1.example.com"}
		scope2 := &types.ClusterScope{Host: "api2.example.com"}

		hash1 := computeScopeHash(scope1)
		hash2 := computeScopeHash(scope2)

		assert.NotEqual(t, hash1, hash2, "different scopes should generate different hashes")
	})
}
