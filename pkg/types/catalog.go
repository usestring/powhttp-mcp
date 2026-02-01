package types

import "encoding/json"

// EndpointCategory classifies an endpoint cluster by its role.
type EndpointCategory string

const (
	CategoryAPI   EndpointCategory = "api"
	CategoryPage  EndpointCategory = "page"
	CategoryAsset EndpointCategory = "asset"
	CategoryData  EndpointCategory = "data"
	CategoryOther EndpointCategory = "other"
)

// ClusterStats provides lightweight statistics for a cluster.
type ClusterStats struct {
	StatusProfile map[string]int `json:"status_profile"`  // e.g. {"2xx": 95, "4xx": 3}
	ErrorRate     float64        `json:"error_rate"`       // fraction of non-2xx (0.0-1.0)
	AvgRespBytes  int            `json:"avg_resp_bytes"`   // average response body size
	HasAuth       bool           `json:"has_auth"`         // any entry had auth signals
}

// ClusterKey is the composite key for clustering: host + method + path_template.
type ClusterKey struct {
	Host         string
	Method       string
	PathTemplate string
}

// Cluster represents a single endpoint cluster.
type Cluster struct {
	ID              string           `json:"cluster_id"`
	Host            string           `json:"host"`
	Method          string           `json:"method"`
	PathTemplate    string           `json:"path_template"`
	Count           int              `json:"count"`
	Category        EndpointCategory `json:"category"`
	Stats           ClusterStats     `json:"stats"`
	ExampleEntryIDs []string         `json:"example_entry_ids"`
	ContentTypeHint string           `json:"content_type_hint,omitempty"`
}

// ExtractRequest contains parameters for cluster extraction.
type ExtractRequest struct {
	SessionID string
	Scope     *ClusterScope
	Filters   *ClusterFilters
	Options   *ClusterOptions
	Limit     int // Default 50, for returned clusters
	Offset    int
}

// ClusterScope defines pre-clustering filters that narrow the input entries.
type ClusterScope struct {
	Host         string
	Method       string // Filter entries by HTTP method before clustering
	ProcessName  string
	PID          int
	TimeWindowMs int64
	SinceMs      int64
	UntilMs      int64
}

// ClusterFilters defines post-clustering filters that narrow the output clusters.
type ClusterFilters struct {
	Category EndpointCategory // Filter clusters by category
	MinCount int              // Minimum requests per cluster
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
	RequestBodyShape  json.RawMessage `json:"request_body_shape,omitempty"`
	ResponseBodyShape json.RawMessage `json:"response_body_shape,omitempty"`
	Examples          []ExampleEntry `json:"examples"`
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

// ExampleEntry contains a sample entry from the cluster.
type ExampleEntry struct {
	EntryID string        `json:"entry_id"`
	Summary *EntrySummary `json:"summary"`
}
