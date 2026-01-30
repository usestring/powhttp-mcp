package contenttype

import (
	"mime"
	"strings"
	"unicode/utf8"
)

// Category represents a broad content-type classification.
type Category string

const (
	JSON   Category = "json"
	XML    Category = "xml"
	HTML   Category = "html"
	YAML   Category = "yaml"
	CSV    Category = "csv"
	Form   Category = "form"
	Text   Category = "text"
	Binary Category = "binary"
)

// Classify returns the broad content category for a content-type header value.
// Uses mime.ParseMediaType to strip parameters (charset, boundary, etc.)
// before matching. Falls back to strings.ToLower for malformed values.
// Returns Binary for empty content-type strings.
func Classify(contentType string) Category {
	if contentType == "" {
		return Binary
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(contentType))
	}

	// JSON: application/json, application/vnd.*+json, any containing "json"
	if strings.Contains(mediaType, "json") {
		return JSON
	}

	// HTML: text/html, application/xhtml+xml
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return HTML
	}

	// XML: application/xml, text/xml, application/vnd.*+xml, any containing "xml"
	if strings.Contains(mediaType, "xml") {
		return XML
	}

	// YAML: application/yaml, text/yaml, application/x-yaml
	if strings.Contains(mediaType, "yaml") {
		return YAML
	}

	// CSV: text/csv, text/tab-separated-values
	if mediaType == "text/csv" || mediaType == "text/tab-separated-values" {
		return CSV
	}

	// Form: application/x-www-form-urlencoded
	if mediaType == "application/x-www-form-urlencoded" {
		return Form
	}

	// Text: text/* subtypes
	if strings.HasPrefix(mediaType, "text/") {
		return Text
	}

	// Binary: image/*, audio/*, video/*, octet-stream, pdf, gzip, zip
	if strings.HasPrefix(mediaType, "image/") ||
		strings.HasPrefix(mediaType, "audio/") ||
		strings.HasPrefix(mediaType, "video/") ||
		strings.Contains(mediaType, "octet-stream") ||
		strings.Contains(mediaType, "pdf") ||
		strings.Contains(mediaType, "gzip") ||
		strings.Contains(mediaType, "zip") {
		return Binary
	}

	return Binary
}

// IsBinary returns true if the content type indicates binary content.
// Falls back to UTF-8 validation when contentType is empty or unrecognized
// and data is provided.
func IsBinary(contentType string, data []byte) bool {
	ct := strings.ToLower(contentType)

	// Known text content types
	if strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") ||
		strings.Contains(ct, "javascript") ||
		strings.Contains(ct, "html") ||
		strings.Contains(ct, "css") ||
		strings.Contains(ct, "yaml") ||
		strings.Contains(ct, "form-urlencoded") {
		return false
	}

	// Known binary content types
	if strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "audio/") ||
		strings.HasPrefix(ct, "video/") ||
		strings.Contains(ct, "octet-stream") ||
		strings.Contains(ct, "gzip") ||
		strings.Contains(ct, "zip") ||
		strings.Contains(ct, "pdf") {
		return true
	}

	// Unknown or empty content type: fall back to UTF-8 validation
	return !utf8.Valid(data)
}

// IsJSON returns true if the content type indicates JSON (case-insensitive).
func IsJSON(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "json")
}
