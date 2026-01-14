package types

// ClusterKey is the composite key for clustering: host + method + path_template.
type ClusterKey struct {
	Host         string
	Method       string
	PathTemplate string
}

// Cluster represents a single endpoint cluster.
type Cluster struct {
	ID              string   `json:"cluster_id"`
	Host            string   `json:"host"`
	Method          string   `json:"method"`
	PathTemplate    string   `json:"path_template"`
	Count           int      `json:"count"`
	ExampleEntryIDs []string `json:"example_entry_ids"`
}

// ExtractRequest contains parameters for cluster extraction.
type ExtractRequest struct {
	SessionID string
	Scope     *ClusterScope
	Options   *ClusterOptions
	Limit     int // Default 50, for returned clusters
	Offset    int
}

// ClusterScope defines the filtering scope for clustering.
type ClusterScope struct {
	Host         string
	ProcessName  string
	PID          int
	TimeWindowMs int64
	SinceMs      int64
	UntilMs      int64
}

// ClusterOptions defines clustering behavior options.
type ClusterOptions struct {
	NormalizeIDs           bool // Default true - convert numeric/uuid segments
	StripVolatileQueryKeys bool // Default true
	ExamplesPerCluster     int  // Default 3, max 10
	MaxClusters            int  // Default 200, max 2000
}

// ExtractResponse contains the extraction results.
type ExtractResponse struct {
	Clusters   []Cluster `json:"clusters"`
	TotalCount int       `json:"total_count"`
	ScopeHash  string    `json:"scope_hash"`
}

// DescribeRequest contains parameters for endpoint description.
type DescribeRequest struct {
	ClusterID   string
	SessionID   string
	MaxExamples int // Default 5
}

// EndpointDescription contains detailed information about an endpoint cluster.
type EndpointDescription struct {
	ClusterID         string             `json:"cluster_id"`
	Host              string             `json:"host"`
	Method            string             `json:"method"`
	PathTemplate      string             `json:"path_template"`
	Count             int                `json:"count"`
	TypicalHeaders    []HeaderFrequency  `json:"typical_headers"`
	AuthSignals       AuthSignals        `json:"auth_signals"`
	QueryKeys         QueryKeyAnalysis   `json:"query_keys"`
	RequestBodySchema *RequestBodySchema `json:"request_body_schema,omitempty"`
	Examples          []ExampleEntry     `json:"examples"`
}

// HeaderFrequency tracks how often a header appears.
type HeaderFrequency struct {
	Name      string  `json:"name"`
	Frequency float64 `json:"frequency"` // 0.0 to 1.0
}

// AuthSignals indicates authentication patterns detected.
type AuthSignals struct {
	CookiesPresent    bool     `json:"cookies_present"`
	BearerPresent     bool     `json:"bearer_present"`
	CustomAuthHeaders []string `json:"custom_auth_headers,omitempty"`
}

// QueryKeyAnalysis categorizes query parameters.
type QueryKeyAnalysis struct {
	Stable   []string `json:"stable"`   // Keys present in most requests
	Volatile []string `json:"volatile"` // Keys that vary (timestamps, nonces)
}

// RequestBodySchema describes the JSON Schema of request bodies.
// Schema is stored as any to avoid schema inference issues with json.RawMessage.
type RequestBodySchema struct {
	Schema      any    `json:"schema"`                 // JSON Schema (Draft 2020-12) as a JSON object
	SampleCount int    `json:"sample_count"`           // Number of samples used
	AllMatch    bool   `json:"all_match"`              // True if all samples had identical schema
	ContentType string `json:"content_type,omitempty"` // Content-Type of the bodies
}

// ExampleEntry contains a sample entry from the cluster.
type ExampleEntry struct {
	EntryID string        `json:"entry_id"`
	Summary *EntrySummary `json:"summary"`
}
