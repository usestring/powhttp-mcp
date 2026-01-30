package textquery

// QueryResult holds extraction results from a single body.
type QueryResult struct {
	Values []any    `json:"values"`
	Count  int      `json:"count"`
	Mode   string   `json:"mode"`
	Errors []string `json:"errors,omitempty"`
}
