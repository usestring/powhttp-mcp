// Package shape provides unified body shape analysis with content-type dispatch.
// It follows the same engine pattern as pkg/textquery for query dispatch.
package shape

import (
	"github.com/invopop/jsonschema"

	js "github.com/usestring/powhttp-mcp/pkg/jsonschema"
)

// Result is a union envelope for shape analysis output.
// The ContentCategory field indicates which shape engine was used,
// and the corresponding format-specific field is populated.
type Result struct {
	ContentCategory string `json:"content_category"` // json, yaml, xml, csv, html, form

	// JSON/YAML fields
	Schema      *jsonschema.Schema `json:"schema,omitempty"`
	FieldStats  []js.FieldStat     `json:"field_stats,omitempty"`
	SampleCount int                `json:"sample_count,omitempty"`
	AllMatch    bool               `json:"all_match,omitempty"`

	// XML fields
	XMLHierarchy *XMLElementHierarchy `json:"xml_hierarchy,omitempty"`

	// CSV fields
	CSVColumns *CSVColumnStats `json:"csv_columns,omitempty"`

	// HTML fields
	HTMLOutline *HTMLDOMOutline `json:"html_outline,omitempty"`

	// Form fields
	FormKeys []FormKeyStat `json:"form_keys,omitempty"`

	// Skip info (for binary or unsupported types)
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
}

// XMLElementHierarchy represents the structural outline of an XML document.
type XMLElementHierarchy struct {
	Root         *XMLElement `json:"root"`
	MaxDepth     int         `json:"max_depth"`
	Truncated    bool        `json:"truncated,omitempty"`
	SampleCount  int         `json:"sample_count"`
}

// XMLElement represents a single element in the XML hierarchy.
type XMLElement struct {
	Name       string        `json:"name"`
	Attributes []string      `json:"attributes,omitempty"`
	Children   []*XMLElement `json:"children,omitempty"`
	ChildCount int           `json:"child_count"`
	Repeated   bool          `json:"repeated,omitempty"` // Appears multiple times as sibling
}

// CSVColumnStats represents the column structure of a CSV document.
type CSVColumnStats struct {
	Columns     []CSVColumn `json:"columns"`
	RowCount    int         `json:"row_count"`
	HasHeaders  bool        `json:"has_headers"`
	SampleCount int         `json:"sample_count"`
}

// CSVColumn describes a single CSV column.
type CSVColumn struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`                      // string, number, boolean
	Format         string   `json:"format,omitempty"`          // uuid, iso8601, url, email, enum
	EmptyFrequency float64  `json:"empty_frequency"`           // Fraction of null/empty values
	Examples       []string `json:"examples,omitempty"`        // Up to 3 example values
	EnumValues     []string `json:"enum_values,omitempty"`     // When format is "enum"
}

// HTMLDOMOutline represents the structural summary of an HTML document.
type HTMLDOMOutline struct {
	Title       string            `json:"title,omitempty"`
	TagCounts   map[string]int    `json:"tag_counts"`
	ElementIDs  []HTMLElementID   `json:"element_ids,omitempty"`
	Forms       []HTMLFormOutline `json:"forms,omitempty"`
	MetaTags    []HTMLMetaTag     `json:"meta_tags,omitempty"`
	Truncated   bool              `json:"truncated,omitempty"`
	SampleCount int               `json:"sample_count"`
}

// HTMLElementID records an element with an id attribute.
type HTMLElementID struct {
	Tag string `json:"tag"`
	ID  string `json:"id"`
}

// HTMLFormOutline describes a form element in an HTML document.
type HTMLFormOutline struct {
	Action string          `json:"action,omitempty"`
	Method string          `json:"method,omitempty"`
	Inputs []HTMLFormInput `json:"inputs,omitempty"`
}

// HTMLFormInput describes an input element within a form.
type HTMLFormInput struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// HTMLMetaTag describes a meta element.
type HTMLMetaTag struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

// FormKeyStat describes a form field in form-urlencoded bodies.
type FormKeyStat struct {
	Key       string   `json:"key"`
	Frequency float64  `json:"frequency"` // 0.0-1.0
	Examples  []string `json:"examples,omitempty"`
}
