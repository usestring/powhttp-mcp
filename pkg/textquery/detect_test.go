package textquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectMode(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        string
	}{
		{"html", "text/html", ModeCSS},
		{"html with charset", "text/html; charset=utf-8", ModeCSS},
		{"xhtml", "application/xhtml+xml", ModeCSS},
		{"xml", "application/xml", ModeXPath},
		{"text/xml", "text/xml", ModeXPath},
		{"vendor xml", "application/vnd.foo+xml", ModeXPath},
		{"form", "application/x-www-form-urlencoded", ModeForm},
		{"yaml", "application/yaml", ModeJQ},
		{"text/yaml", "text/yaml", ModeJQ},
		{"x-yaml", "application/x-yaml", ModeJQ},
		{"json", "application/json", ModeJQ},
		{"json with charset", "application/json; charset=utf-8", ModeJQ},
		{"vendor json", "application/vnd.api+json", ModeJQ},
		{"plain text", "text/plain", ModeRegex},
		{"csv", "text/csv", ModeRegex},
		{"unknown", "application/octet-stream", ModeRegex},
		{"empty", "", ModeRegex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMode(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}
