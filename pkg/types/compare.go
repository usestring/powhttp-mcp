package types

// Fingerprint represents a canonical fingerprint for an HTTP entry.
type Fingerprint struct {
	Entry              *EntrySummary       `json:"entry"`
	HeadersOrdered     [][]string          `json:"headers_ordered"`
	HeadersNormalized  map[string][]string `json:"headers_normalized"`
	HTTP2PseudoHeaders [][]string          `json:"http2_pseudo_headers,omitempty"`
	Body               BodyFingerprint     `json:"body"`
	TLSSummary         *TLSFingerprint     `json:"tls_summary,omitempty"`
	HTTP2Summary       *HTTP2Fingerprint   `json:"http2_summary,omitempty"`
}

// BodyFingerprint contains SHA256 hashes and byte counts for request/response bodies.
type BodyFingerprint struct {
	ReqHash   string `json:"req_hash,omitempty"`
	ReqBytes  int    `json:"req_bytes"`
	RespHash  string `json:"resp_hash,omitempty"`
	RespBytes int    `json:"resp_bytes"`
}

// TLSFingerprint contains TLS connection details for fingerprint comparison.
type TLSFingerprint struct {
	ConnectionID string `json:"connection_id,omitempty"`
	TLSVersion   string `json:"tls_version,omitempty"`
	CipherSuite  string `json:"cipher_suite,omitempty"`
	JA3          string `json:"ja3,omitempty"`
	JA4          string `json:"ja4,omitempty"`
	ALPN         string `json:"alpn,omitempty"`
}

// HTTP2Fingerprint contains HTTP/2 connection details for fingerprint comparison.
type HTTP2Fingerprint struct {
	ConnectionID string         `json:"connection_id,omitempty"`
	StreamID     int            `json:"stream_id,omitempty"`
	FrameCounts  map[string]int `json:"frame_counts,omitempty"`
}

// FingerprintOptions controls fingerprint generation behavior.
type FingerprintOptions struct {
	IncludeTLSSummary   bool // Default true
	IncludeHTTP2Summary bool // Default true
	MaxBytes            int  // Default from config.ToolMaxBytesDefault
}

// DiffRequest contains parameters for comparing two entries.
type DiffRequest struct {
	BaselineEntryID  string
	CandidateEntryID string
	SessionID        string // Default "active"
	Options          *DiffOptions
}

// DiffOptions controls diff behavior.
type DiffOptions struct {
	CompareHeaderOrder  bool     // Default true
	CompareHeaderValues bool     // Default true
	CompareTLS          bool     // Default true
	CompareHTTP2        bool     // Default true
	IgnoreHeaders       []string // Default from DefaultIgnoreHeaders
	IgnoreQueryKeys     []string // Default from DefaultIgnoreQueryKeys
	MaxBytes            int      // Default from config
}

// DiffResult contains the structured comparison of two entries.
type DiffResult struct {
	Baseline       *EntrySummary  `json:"baseline"`
	Candidate      *EntrySummary  `json:"candidate"`
	ImportantDiffs ImportantDiffs `json:"important_diffs"`
	NoisyDiffs     NoisyDiffs     `json:"noisy_diffs"`
}

// ImportantDiffs contains differences that are likely meaningful for anti-bot detection.
type ImportantDiffs struct {
	Protocol            *ProtocolDiff     `json:"protocol,omitempty"`
	TLS                 *TLSDiff          `json:"tls,omitempty"`
	HTTP2               *HTTP2Diff        `json:"http2,omitempty"`
	HeadersMissing      []string          `json:"headers_missing,omitempty"`
	HeadersExtra        []string          `json:"headers_extra,omitempty"`
	HeadersValueChanged []HeaderValueDiff `json:"headers_value_changed,omitempty"`
	HeaderOrderChanges  *HeaderOrderDiff  `json:"header_order_changes,omitempty"`
}

// NoisyDiffs contains differences that are typically not meaningful.
type NoisyDiffs struct {
	IgnoredHeaders []string `json:"ignored_headers,omitempty"`
	QueryKeyDiffs  []string `json:"query_key_diffs,omitempty"`
}

// ProtocolDiff represents a difference in HTTP protocol version.
type ProtocolDiff struct {
	BaselineVersion  string `json:"baseline_version"`
	CandidateVersion string `json:"candidate_version"`
}

// TLSDiff represents differences in TLS fingerprints.
type TLSDiff struct {
	JA3Different     bool   `json:"ja3_different"`
	JA4Different     bool   `json:"ja4_different"`
	BaselineJA3      string `json:"baseline_ja3,omitempty"`
	CandidateJA3     string `json:"candidate_ja3,omitempty"`
	BaselineJA4      string `json:"baseline_ja4,omitempty"`
	CandidateJA4     string `json:"candidate_ja4,omitempty"`
	CipherDifferent  bool   `json:"cipher_different,omitempty"`
	VersionDifferent bool   `json:"version_different,omitempty"`
}

// HTTP2Diff represents differences in HTTP/2 metadata.
type HTTP2Diff struct {
	BaselineStreamID  int  `json:"baseline_stream_id"`
	CandidateStreamID int  `json:"candidate_stream_id"`
	PseudoHeadersDiff bool `json:"pseudo_headers_diff"`
}

// HeaderValueDiff represents a difference in header values.
type HeaderValueDiff struct {
	Name      string   `json:"name"`
	Baseline  []string `json:"baseline"`
	Candidate []string `json:"candidate"`
}

// HeaderOrderDiff represents differences in header ordering.
type HeaderOrderDiff struct {
	BaselineOrder  []string      `json:"baseline_order"`
	CandidateOrder []string      `json:"candidate_order"`
	Moves          []OrderChange `json:"moves,omitempty"`
}

// OrderChange represents a header that moved position.
type OrderChange struct {
	Header       string `json:"header"`
	BaselinePos  int    `json:"baseline_pos"`
	CandidatePos int    `json:"candidate_pos"`
}
