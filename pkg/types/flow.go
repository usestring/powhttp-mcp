package types

// Edge reason constants for flow graphs.
const (
	EdgeReasonTemporal            = "temporal"
	EdgeReasonSameTLS             = "same_tls"
	EdgeReasonSameH2              = "same_h2"
	EdgeReasonAuthChain           = "auth_chain"
	EdgeReasonSameAuth            = "same_auth"             // Same Authorization header value
	EdgeReasonSameSessionCookie   = "same_session_cookie"   // Same session cookie
	EdgeReasonSameAPIKey          = "same_api_key"          // Same API key header
	EdgeReasonSessionCookieOrigin = "session_cookie_origin" // Set-Cookie -> Cookie chain
)

// TraceRequest contains parameters for flow tracing.
type TraceRequest struct {
	SessionID   string
	SeedEntryID string
	MaxDepth    int // Default 50, max 500
	Options     *TraceOptions
	Limit       int // Default 50 for returned nodes
}

// TraceOptions defines flow tracing behavior options.
type TraceOptions struct {
	TimeWindowMs int64 // Default 120000 (2 minutes)
	SamePIDOnly  bool  // Default true
	SameHostOnly bool  // Default true
}

// FlowGraph represents a graph of related HTTP requests.
type FlowGraph struct {
	Nodes []FlowNode `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
}

// FlowNode represents a single node in the flow graph.
type FlowNode struct {
	EntryID string        `json:"entry_id"`
	Summary *EntrySummary `json:"summary"`
}

// FlowEdge represents a relationship between two entries.
type FlowEdge struct {
	From   string `json:"from"`   // Entry ID
	To     string `json:"to"`     // Entry ID
	Reason string `json:"reason"` // temporal, same_tls, same_h2, auth_chain, same_auth, same_session_cookie, same_api_key, session_cookie_origin
}
