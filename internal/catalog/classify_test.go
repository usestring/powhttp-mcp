package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestClassifyCluster(t *testing.T) {
	tests := []struct {
		name         string
		key          types.ClusterKey
		contentTypes map[string]int
		expected     types.EndpointCategory
	}{
		// API endpoints
		{
			name:         "JSON API",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/api/users/{id}"},
			contentTypes: map[string]int{"application/json": 50},
			expected:     types.CategoryAPI,
		},
		{
			name:         "XML API",
			key:          types.ClusterKey{Host: "api.example.com", Method: "POST", PathTemplate: "/soap/service"},
			contentTypes: map[string]int{"application/xml": 20},
			expected:     types.CategoryAPI,
		},
		{
			name:         "API path pattern with no content type",
			key:          types.ClusterKey{Host: "api.example.com", Method: "DELETE", PathTemplate: "/api/users/{id}"},
			contentTypes: map[string]int{},
			expected:     types.CategoryAPI,
		},
		{
			name:         "versioned API path",
			key:          types.ClusterKey{Host: "api.example.com", Method: "POST", PathTemplate: "/v2/auth/login"},
			contentTypes: map[string]int{},
			expected:     types.CategoryAPI,
		},
		{
			name:         "REST path",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/rest/items"},
			contentTypes: map[string]int{},
			expected:     types.CategoryAPI,
		},
		{
			name:         "GraphQL path",
			key:          types.ClusterKey{Host: "api.example.com", Method: "POST", PathTemplate: "/graphql"},
			contentTypes: map[string]int{"application/json": 100},
			expected:     types.CategoryAPI,
		},
		{
			name:         "JSON with charset",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/data"},
			contentTypes: map[string]int{"application/json; charset=utf-8": 30},
			expected:     types.CategoryAPI,
		},
		{
			name:         "mixed JSON and HTML prefers JSON dominant",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/endpoint"},
			contentTypes: map[string]int{"application/json": 80, "text/html": 20},
			expected:     types.CategoryAPI,
		},

		// Page endpoints
		{
			name:         "HTML page",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/about"},
			contentTypes: map[string]int{"text/html": 10},
			expected:     types.CategoryPage,
		},
		{
			name:         "XHTML page",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/"},
			contentTypes: map[string]int{"application/xhtml+xml": 5},
			expected:     types.CategoryPage,
		},

		// Asset endpoints
		{
			name:         "JavaScript file",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/static/app.js"},
			contentTypes: map[string]int{"application/javascript": 1},
			expected:     types.CategoryAsset,
		},
		{
			name:         "CSS file",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/styles/main.css"},
			contentTypes: map[string]int{"text/css": 1},
			expected:     types.CategoryAsset,
		},
		{
			name:         "PNG image",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/images/logo.png"},
			contentTypes: map[string]int{"image/png": 3},
			expected:     types.CategoryAsset,
		},
		{
			name:         "font file",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/fonts/roboto.woff2"},
			contentTypes: map[string]int{"font/woff2": 1},
			expected:     types.CategoryAsset,
		},
		{
			name:         "source map",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/app.js.map"},
			contentTypes: map[string]int{"application/json": 1},
			expected:     types.CategoryAsset,
		},
		{
			name:         "image by content type no extension",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/img/{id}"},
			contentTypes: map[string]int{"image/webp": 50},
			expected:     types.CategoryAsset,
		},
		{
			name:         "JS by content type no extension",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/bundle/{hex}"},
			contentTypes: map[string]int{"application/javascript": 10},
			expected:     types.CategoryAsset,
		},
		{
			name:         "Next.js asset path",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/_next/static/chunks/{hex}"},
			contentTypes: map[string]int{},
			expected:     types.CategoryAsset,
		},
		{
			name:         "static directory path",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/static/media/{hex}"},
			contentTypes: map[string]int{},
			expected:     types.CategoryAsset,
		},
		{
			name:         "SVG file",
			key:          types.ClusterKey{Host: "cdn.example.com", Method: "GET", PathTemplate: "/icons/close.svg"},
			contentTypes: map[string]int{"image/svg+xml": 1},
			expected:     types.CategoryAsset,
		},
		{
			name:         "PDF file",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/docs/report.pdf"},
			contentTypes: map[string]int{"application/pdf": 1},
			expected:     types.CategoryAsset,
		},

		// Data endpoints
		{
			name:         "CSV download",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/export/report.csv"},
			contentTypes: map[string]int{"text/csv": 5},
			expected:     types.CategoryData,
		},
		{
			name:         "form data",
			key:          types.ClusterKey{Host: "example.com", Method: "POST", PathTemplate: "/submit"},
			contentTypes: map[string]int{"application/x-www-form-urlencoded": 10},
			expected:     types.CategoryData,
		},

		// Other / fallback
		{
			name:         "no content type no path hints",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/unknown"},
			contentTypes: map[string]int{},
			expected:     types.CategoryOther,
		},
		{
			name:         "root path with no content type",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: "/"},
			contentTypes: map[string]int{},
			expected:     types.CategoryOther,
		},

		// Edge cases
		{
			name:         "API path with asset extension prefers asset",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/api/v1/logo.png"},
			contentTypes: map[string]int{"image/png": 10},
			expected:     types.CategoryAsset,
		},
		{
			name:         "path with template param last segment",
			key:          types.ClusterKey{Host: "api.example.com", Method: "GET", PathTemplate: "/items/{id}"},
			contentTypes: map[string]int{"application/json": 20},
			expected:     types.CategoryAPI,
		},
		{
			name:         "empty path",
			key:          types.ClusterKey{Host: "example.com", Method: "GET", PathTemplate: ""},
			contentTypes: map[string]int{},
			expected:     types.CategoryOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyCluster(tt.key, tt.contentTypes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDominantContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]int
		expected string
	}{
		{"empty map", map[string]int{}, ""},
		{"single type", map[string]int{"application/json": 10}, "application/json"},
		{"dominant type", map[string]int{"application/json": 80, "text/html": 20}, "application/json"},
		{"tied picks one", map[string]int{"text/html": 5, "application/json": 5}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dominantContentType(tt.input)
			if tt.name == "tied picks one" {
				// Either is acceptable
				assert.Contains(t, []string{"text/html", "application/json"}, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLastPathSegment(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple file", "/static/app.js", "/app.js"},
		{"template param", "/users/{id}", ""},
		{"root", "/", "/"},
		{"no slash", "file.js", "file.js"},
		{"nested file", "/a/b/c.css", "/c.css"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lastPathSegment(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAssetPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/static/main.js", true},
		{"/assets/logo.png", true},
		{"/dist/bundle.js", true},
		{"/_next/static/chunks/abc", true},
		{"/api/users", false},
		{"/about", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isAssetPath(tt.path))
		})
	}
}
