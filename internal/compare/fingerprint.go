package compare

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/types"
)


// FingerprintEngine generates fingerprints for HTTP entries.
type FingerprintEngine struct {
	client *client.Client
	cache  *cache.EntryCache
	config *config.Config
}


// NewFingerprintEngine creates a new FingerprintEngine.
func NewFingerprintEngine(c *client.Client, entryCache *cache.EntryCache, cfg *config.Config) *FingerprintEngine {
	return &FingerprintEngine{
		client: c,
		cache:  entryCache,
		config: cfg,
	}
}

// Generate creates a fingerprint for an entry.
func (f *FingerprintEngine) Generate(ctx context.Context, sessionID, entryID string, opts *types.FingerprintOptions) (*types.Fingerprint, error) {
	if opts == nil {
		opts = &types.FingerprintOptions{
			IncludeTLSSummary:   true,
			IncludeHTTP2Summary: true,
			MaxBytes:            f.config.ToolMaxBytesDefault,
		}
	}

	// Fetch entry from cache or API
	entry, err := f.getEntry(ctx, sessionID, entryID)
	if err != nil {
		return nil, fmt.Errorf("fetching entry: %w", err)
	}

	// Build entry summary
	summary := buildEntrySummary(entry)

	// Extract headers
	headersOrdered := extractOrderedHeaders(entry.Request.Headers)
	headersNormalized := normalizeHeaders(entry.Request.Headers)

	// Extract HTTP/2 pseudo-headers if applicable
	var pseudoHeaders [][]string
	if entry.HTTP2 != nil {
		pseudoHeaders = extractPseudoHeaders(entry.Request.Headers)
	}

	// Compute body fingerprints
	bodyFP := computeBodyFingerprint(entry, opts.MaxBytes)

	fp := &types.Fingerprint{
		Entry:              summary,
		HeadersOrdered:     headersOrdered,
		HeadersNormalized:  headersNormalized,
		HTTP2PseudoHeaders: pseudoHeaders,
		Body:               bodyFP,
	}

	// Fetch TLS summary if requested and available
	if opts.IncludeTLSSummary && entry.TLS.ConnectionID != nil {
		tlsSummary, err := f.fetchTLSSummary(ctx, *entry.TLS.ConnectionID, entry)
		if err == nil {
			fp.TLSSummary = tlsSummary
		}
	}

	// Fetch HTTP/2 summary if requested and available
	if opts.IncludeHTTP2Summary && entry.HTTP2 != nil {
		h2Summary, err := f.fetchHTTP2Summary(ctx, entry.HTTP2.ConnectionID, entry.HTTP2.StreamID)
		if err == nil {
			fp.HTTP2Summary = h2Summary
		}
	}

	return fp, nil
}

// getEntry fetches an entry from cache or API.
func (f *FingerprintEngine) getEntry(ctx context.Context, sessionID, entryID string) (*client.SessionEntry, error) {
	// Check cache first
	if entry, ok := f.cache.Get(entryID); ok {
		return entry, nil
	}

	// Fetch from API
	entry, err := f.client.GetEntry(ctx, sessionID, entryID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	f.cache.Put(entryID, entry)

	return entry, nil
}

// buildEntrySummary creates an EntrySummary from a SessionEntry.
func buildEntrySummary(entry *client.SessionEntry) *types.EntrySummary {
	summary := &types.EntrySummary{
		EntryID:     entry.ID,
		TsMs:        entry.Timings.StartedAt,
		URL:         entry.URL,
		HTTPVersion: entry.HTTPVersion,
	}

	// Parse host and path from URL
	if parsed, err := url.Parse(entry.URL); err == nil {
		summary.Host = strings.ToLower(parsed.Host)
		summary.Path = parsed.Path
	}

	// Request fields
	if entry.Request.Method != nil {
		summary.Method = *entry.Request.Method
	}

	// Response fields
	if entry.Response != nil && entry.Response.StatusCode != nil {
		summary.Status = *entry.Response.StatusCode
	}

	// Process info
	if entry.Process != nil {
		summary.PID = entry.Process.PID
		if entry.Process.Name != nil {
			summary.ProcessName = *entry.Process.Name
		}
	}

	// TLS info
	if entry.TLS.ConnectionID != nil {
		summary.TLS.ConnectionID = *entry.TLS.ConnectionID
	}
	if entry.TLS.JA3 != nil {
		summary.TLS.JA3 = entry.TLS.JA3.Hash
	}
	if entry.TLS.JA4 != nil {
		summary.TLS.JA4 = entry.TLS.JA4.Hashed
	}

	// HTTP/2 info
	if entry.HTTP2 != nil {
		summary.HTTP2.ConnectionID = entry.HTTP2.ConnectionID
		summary.HTTP2.StreamID = entry.HTTP2.StreamID
	}

	// Body sizes
	summary.Sizes.ReqBodyBytes = computeBodySize(entry.Request.Body)
	if entry.Response != nil {
		summary.Sizes.RespBodyBytes = computeBodySize(entry.Response.Body)
	}

	return summary
}

// extractOrderedHeaders preserves the original header order.
func extractOrderedHeaders(headers client.Headers) [][]string {
	result := make([][]string, 0, len(headers))
	for _, pair := range headers {
		if len(pair) >= 2 {
			result = append(result, []string{pair[0], pair[1]})
		}
	}
	return result
}

// normalizeHeaders creates a lowercase key -> values map.
func normalizeHeaders(headers client.Headers) map[string][]string {
	result := make(map[string][]string)
	for _, pair := range headers {
		if len(pair) >= 2 {
			key := strings.ToLower(pair[0])
			result[key] = append(result[key], pair[1])
		}
	}
	return result
}

// extractPseudoHeaders extracts HTTP/2 pseudo-headers (starting with ':').
func extractPseudoHeaders(headers client.Headers) [][]string {
	var result [][]string
	for _, pair := range headers {
		if len(pair) >= 2 && strings.HasPrefix(pair[0], ":") {
			result = append(result, []string{pair[0], pair[1]})
		}
	}
	return result
}

// computeBodyFingerprint computes hashes and sizes for request/response bodies.
func computeBodyFingerprint(entry *client.SessionEntry, maxBytes int) types.BodyFingerprint {
	fp := types.BodyFingerprint{}

	// Request body
	if entry.Request.Body != nil {
		fp.ReqBytes = computeBodySize(entry.Request.Body)
		if fp.ReqBytes > 0 && fp.ReqBytes <= maxBytes {
			fp.ReqHash = hashBody(entry.Request.Body)
		}
	}

	// Response body
	if entry.Response != nil && entry.Response.Body != nil {
		fp.RespBytes = computeBodySize(entry.Response.Body)
		if fp.RespBytes > 0 && fp.RespBytes <= maxBytes {
			fp.RespHash = hashBody(entry.Response.Body)
		}
	}

	return fp
}

// hashBody computes SHA256 of a decoded base64 body.
func hashBody(encoded *string) string {
	if encoded == nil || *encoded == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(decoded)
	return hex.EncodeToString(hash[:])
}

// computeBodySize calculates body size from base64 encoded string.
func computeBodySize(encoded *string) int {
	if encoded == nil || *encoded == "" {
		return 0
	}

	decoded, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		// Estimate from base64 length
		n := len(*encoded)
		padding := strings.Count((*encoded)[max(0, n-2):], "=")
		return (n * 3 / 4) - padding
	}

	return len(decoded)
}

// fetchTLSSummary gets TLS connection details using typed TLSEvent structures.
func (f *FingerprintEngine) fetchTLSSummary(ctx context.Context, connectionID string, entry *client.SessionEntry) (*types.TLSFingerprint, error) {
	summary := &types.TLSFingerprint{
		ConnectionID: connectionID,
	}

	// Get JA3/JA4 from entry if available
	if entry.TLS.JA3 != nil {
		summary.JA3 = entry.TLS.JA3.Hash
	}
	if entry.TLS.JA4 != nil {
		summary.JA4 = entry.TLS.JA4.Hashed
	}

	// Fetch TLS events for additional details
	events, err := f.client.GetTLSConnection(ctx, connectionID)
	if err != nil {
		// Return what we have from entry
		return summary, nil
	}

	// Extract version, cipher suite from typed events
	for _, event := range events {
		if event.Msg.Type == client.TLSMsgHandshake && event.Msg.Handshake != nil {
			switch event.Msg.Handshake.Type {
			case client.TLSHandshakeServerHello:
				if event.Msg.Handshake.ServerHello != nil {
					sh := event.Msg.Handshake.ServerHello
					summary.TLSVersion = sh.Version.Name
					summary.CipherSuite = sh.CipherSuite.Name
				}
			case client.TLSHandshakeClientHello:
				if event.Msg.Handshake.ClientHello != nil {
					ch := event.Msg.Handshake.ClientHello
					if summary.TLSVersion == "" {
						summary.TLSVersion = ch.Version.Name
					}
				}
			}
		}
	}

	return summary, nil
}

// fetchHTTP2Summary gets HTTP/2 stream details from frame data.
func (f *FingerprintEngine) fetchHTTP2Summary(ctx context.Context, connID string, streamID int) (*types.HTTP2Fingerprint, error) {
	summary := &types.HTTP2Fingerprint{
		ConnectionID: connID,
		StreamID:     streamID,
		FrameCounts:  make(map[string]int),
	}

	// Fetch HTTP/2 frames
	frames, err := f.client.GetHTTP2Stream(ctx, connID, streamID)
	if err != nil {
		return summary, nil
	}

	// Count frame types
	for _, raw := range frames {
		var frame struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		if frame.Type != "" {
			summary.FrameCounts[frame.Type]++
		}
	}

	return summary, nil
}
