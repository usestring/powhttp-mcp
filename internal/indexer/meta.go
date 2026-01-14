// Package indexer provides entry metadata and indexing functionality.
package indexer

import (
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// HeaderValue represents a header name:value pair for indexing.
type HeaderValue struct {
	Name  string
	Value string
}

// EntryMeta holds searchable fields for one entry.
// Bodies are not stored; only metadata needed for indexing and search.
type EntryMeta struct {
	DocID            uint32
	EntryID          string
	TsMs             int64
	Method           string
	URL              string
	Host             string
	Path             string
	Status           int
	HTTPVersion      string
	ProcessName      string
	PID              int
	HeaderNamesLower []string
	HeaderValues     []HeaderValue // All header name:value pairs (lowercase names)

	// TLS pointers
	TLSConnectionID string
	JA3             string
	JA4             string

	// HTTP/2 pointers
	H2ConnectionID string
	H2StreamID     int

	// Size tracking
	ReqBodyBytes  int
	RespBodyBytes int

	// Auth fields for flow tracing
	AuthHeader string            // Full Authorization header value (Bearer, Basic, etc.)
	Cookies    map[string]string // Session cookie name -> value (filtered to session-related)
	APIKeys    map[string]string // Auth header name -> value (x-api-key, etc.)
	SetCookies map[string]string // Set-Cookie name -> value (from response, session-related only)
}

// ToSummary converts EntryMeta to EntrySummary for tool responses.
func (m *EntryMeta) ToSummary() *types.EntrySummary {
	return &types.EntrySummary{
		EntryID:     m.EntryID,
		TsMs:        m.TsMs,
		Method:      m.Method,
		URL:         m.URL,
		Host:        m.Host,
		Path:        m.Path,
		Status:      m.Status,
		HTTPVersion: m.HTTPVersion,
		ProcessName: m.ProcessName,
		PID:         m.PID,
		TLS: types.TLSSummary{
			ConnectionID: m.TLSConnectionID,
			JA3:          m.JA3,
			JA4:          m.JA4,
		},
		HTTP2: types.HTTP2Summary{
			ConnectionID: m.H2ConnectionID,
			StreamID:     m.H2StreamID,
		},
		Sizes: types.SizeSummary{
			ReqBodyBytes:  m.ReqBodyBytes,
			RespBodyBytes: m.RespBodyBytes,
		},
	}
}
