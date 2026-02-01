package types

// QueryRequest contains parameters for a body query operation.
type QueryRequest struct {
	SessionID   string
	EntryIDs    []string // Query specific entries
	ClusterID   string   // Or query all entries in a cluster
	Expression  string   // JQ expression
	Target      string   // "request", "response", or "both"
	Deduplicate bool
	MaxEntries  int // Default 20, max 100
	MaxResults  int // Default 1000
}

// QueryResult contains the results of a body query.
type QueryResult struct {
	Values   []any    `json:"values"`           // Extracted values
	Errors   []string `json:"errors,omitempty"` // Per-item errors
	RawCount int      `json:"raw_count"`        // Count before deduplication
}

// QuerySummary contains summary statistics for a query.
type QuerySummary struct {
	EntriesProcessed int  `json:"entries_processed"`
	EntriesMatched   int  `json:"entries_matched"`
	EntriesSkipped   int  `json:"entries_skipped"`
	TotalValues      int  `json:"total_values"`
	UniqueValues     int  `json:"unique_values,omitempty"`
	Deduplicated     bool `json:"deduplicated"`
	Truncated        bool `json:"truncated,omitempty"`
}

// QueryEntryResult contains per-entry query results.
type QueryEntryResult struct {
	EntryID    string        `json:"entry_id"`
	Target     string        `json:"target"`
	ValueCount int           `json:"value_count"`
	Summary    *EntrySummary `json:"summary,omitempty"`
	Skipped    bool          `json:"skipped,omitempty"`
	SkipReason string        `json:"skip_reason,omitempty"`
}

// QueryResponse contains the full response from a body query operation.
type QueryResponse struct {
	Summary QuerySummary       `json:"summary"`
	Values  []any              `json:"values,omitzero"`
	Entries []QueryEntryResult `json:"entries,omitempty"`
	Errors  []string           `json:"errors,omitempty"`
	Hints   []string           `json:"hints,omitempty"`
}
