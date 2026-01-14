package types

// SearchRequest contains parameters for a search query.
type SearchRequest struct {
	SessionID string         // Session to search within
	Query     string         // Free text query
	Filters   *SearchFilters // Optional structured filters
	Limit     int            // Default 20, max 100
	Offset    int            // Pagination offset
}

// SearchFilters contains structured filter criteria.
type SearchFilters struct {
	Host            string
	PathContains    string
	URLContains     string
	Method          string
	Status          int
	HTTPVersion     string
	ProcessName     string
	PID             int
	HeaderName      string
	TLSConnectionID string
	JA3             string
	JA4             string
	SinceMs         int64 // Unix timestamp in ms
	UntilMs         int64 // Unix timestamp in ms
	TimeWindowMs    int64 // Alternative to since/until (relative to now)
}

// SearchResult represents a single search result.
type SearchResult struct {
	Summary    *EntrySummary `json:"summary"`
	Score      float64       `json:"score"`
	Highlights []string      `json:"highlights,omitempty"`
}

// SearchResponse contains the search results.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalHint  int            `json:"total_hint,omitempty"`
	SyncedAtMs int64          `json:"synced_at_ms"`
}
