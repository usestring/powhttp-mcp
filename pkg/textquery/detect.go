package textquery

import (
	"github.com/usestring/powhttp-mcp/pkg/contenttype"
)

// Mode constants for extraction languages.
const (
	ModeCSS   = "css"
	ModeXPath = "xpath"
	ModeRegex = "regex"
	ModeForm  = "form"
	ModeJQ    = "jq"
)

// DetectMode returns the appropriate extraction mode for a content-type header.
func DetectMode(ct string) string {
	switch contenttype.Classify(ct) {
	case contenttype.JSON:
		return ModeJQ
	case contenttype.HTML:
		return ModeCSS
	case contenttype.XML:
		return ModeXPath
	case contenttype.Form:
		return ModeForm
	case contenttype.YAML:
		return ModeJQ
	default:
		return ModeRegex
	}
}
