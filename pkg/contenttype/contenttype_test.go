package contenttype

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        Category
	}{
		// JSON
		{"application/json", "application/json", JSON},
		{"vendor json", "application/vnd.api+json", JSON},
		{"json with charset", "application/json; charset=utf-8", JSON},
		{"json with complex params", "application/json; charset=utf-8; boundary=something", JSON},

		// HTML
		{"text/html", "text/html", HTML},
		{"html with charset", "text/html; charset=utf-8", HTML},
		{"xhtml", "application/xhtml+xml", HTML},

		// XML
		{"application/xml", "application/xml", XML},
		{"text/xml", "text/xml", XML},
		{"vendor xml", "application/vnd.foo+xml", XML},

		// YAML
		{"application/yaml", "application/yaml", YAML},
		{"text/yaml", "text/yaml", YAML},
		{"application/x-yaml", "application/x-yaml", YAML},

		// CSV
		{"text/csv", "text/csv", CSV},
		{"tsv", "text/tab-separated-values", CSV},

		// Form
		{"form-urlencoded", "application/x-www-form-urlencoded", Form},

		// Text
		{"text/plain", "text/plain", Text},
		{"text/javascript", "text/javascript", Text},
		{"text/css", "text/css", Text},
		{"text/markdown", "text/markdown", Text},

		// Binary
		{"image/png", "image/png", Binary},
		{"audio/mp3", "audio/mp3", Binary},
		{"video/mp4", "video/mp4", Binary},
		{"octet-stream", "application/octet-stream", Binary},
		{"pdf", "application/pdf", Binary},
		{"gzip", "application/gzip", Binary},
		{"zip", "application/zip", Binary},

		// Edge cases
		{"empty", "", Binary},
		{"uppercase", "Application/JSON", JSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        []byte
		want        bool
	}{
		// Known text types
		{"json", "application/json", nil, false},
		{"vendor json", "application/vnd.api+json", nil, false},
		{"html", "text/html", nil, false},
		{"xml", "application/xml", nil, false},
		{"text/xml", "text/xml", nil, false},
		{"javascript", "text/javascript", nil, false},
		{"css", "text/css", nil, false},
		{"yaml", "application/yaml", nil, false},
		{"form", "application/x-www-form-urlencoded", nil, false},
		{"text/plain", "text/plain", nil, false},

		// Known binary types
		{"image", "image/png", nil, true},
		{"audio", "audio/mp3", nil, true},
		{"video", "video/mp4", nil, true},
		{"octet-stream", "application/octet-stream", nil, true},
		{"gzip", "application/gzip", nil, true},
		{"zip", "application/zip", nil, true},
		{"pdf", "application/pdf", nil, true},

		// UTF-8 fallback
		{"empty with utf8 data", "", []byte("hello world"), false},
		{"empty with binary data", "", []byte{0xff, 0xfe, 0x00, 0x01}, true},
		{"empty with nil data", "", nil, false}, // utf8.Valid(nil) == true

		// Unknown type with data
		{"unknown with utf8", "application/unknown", []byte("valid text"), false},
		{"unknown with binary", "application/unknown", []byte{0x80, 0x81}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinary(tt.contentType, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{"application/json", "application/json", true},
		{"vendor json", "application/vnd.api+json", true},
		{"vendor json with charset", "application/vnd.api+json; charset=utf-8", true},
		{"html", "text/html", false},
		{"xml", "application/xml", false},
		{"empty", "", false},
		{"uppercase", "Application/JSON", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJSON(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}
