package catalog

import (
	"path"
	"regexp"
	"strings"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// assetExtensions maps file extensions that indicate static assets.
var assetExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true, ".css": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".ico": true, ".webp": true, ".avif": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	".map": true, // source maps
	".mp4": true, ".webm": true, ".mp3": true, ".ogg": true,
	".pdf": true,
}

// apiPathPattern matches common API path prefixes.
var apiPathPattern = regexp.MustCompile(`(?i)^/(api|graphql|rest|v\d+)(/|$)`)

// classifyCluster determines the endpoint category for a cluster based on
// path patterns and response content type distribution.
func classifyCluster(key types.ClusterKey, contentTypes map[string]int) types.EndpointCategory {
	pathLower := strings.ToLower(key.PathTemplate)
	ext := strings.ToLower(path.Ext(lastPathSegment(pathLower)))

	// 1. Asset by file extension (cheapest, most definitive)
	if assetExtensions[ext] {
		return types.CategoryAsset
	}

	// 2. Content-type based classification (only when we have data)
	dominantCT := dominantContentType(contentTypes)
	if dominantCT != "" {
		ctCategory := contenttype.Classify(dominantCT)

		// Asset by content type (for extensionless CDN URLs)
		if ctCategory == contenttype.Binary {
			return types.CategoryAsset
		}
		ctLower := strings.ToLower(dominantCT)
		if strings.Contains(ctLower, "javascript") || strings.Contains(ctLower, "css") {
			return types.CategoryAsset
		}

		// API: structured data content types
		if ctCategory == contenttype.JSON || ctCategory == contenttype.XML || ctCategory == contenttype.YAML {
			return types.CategoryAPI
		}

		// Page: HTML content
		if ctCategory == contenttype.HTML {
			return types.CategoryPage
		}

		// Data: CSV, form data
		if ctCategory == contenttype.CSV || ctCategory == contenttype.Form {
			return types.CategoryData
		}
	}

	// 3. API: path pattern match (catches endpoints returning 204 No Content, etc.)
	if apiPathPattern.MatchString(key.PathTemplate) {
		return types.CategoryAPI
	}

	// 4. Asset by path pattern (common framework static dirs)
	if isAssetPath(pathLower) {
		return types.CategoryAsset
	}

	return types.CategoryOther
}

// dominantContentType returns the most frequent content type from the distribution.
func dominantContentType(contentTypes map[string]int) string {
	var top string
	var topCount int
	for ct, count := range contentTypes {
		if count > topCount {
			top = ct
			topCount = count
		}
	}
	return top
}

// lastPathSegment returns the last segment of a path for extension detection.
// Returns empty string if the last segment is a template parameter like {id}.
func lastPathSegment(p string) string {
	lastSlash := strings.LastIndex(p, "/")
	if lastSlash < 0 {
		return p
	}
	segment := p[lastSlash:]
	trimmed := strings.TrimPrefix(segment, "/")
	if strings.HasPrefix(trimmed, "{") {
		return ""
	}
	return segment
}

// isAssetPath checks for common framework static asset path patterns.
func isAssetPath(pathLower string) bool {
	return strings.Contains(pathLower, "/static/") ||
		strings.Contains(pathLower, "/assets/") ||
		strings.Contains(pathLower, "/dist/") ||
		strings.Contains(pathLower, "/bundle") ||
		strings.Contains(pathLower, "/_next/") ||
		strings.Contains(pathLower, "/chunks/")
}
