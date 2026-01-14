package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// FingerprintInput is the input for powhttp_fingerprint.
type FingerprintInput struct {
	SessionID           string `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	EntryID             string `json:"entry_id" jsonschema:"required,Entry ID to fingerprint"`
	IncludeTLSSummary   bool   `json:"include_tls_summary,omitempty" jsonschema:"Include TLS details (default: true)"`
	IncludeHTTP2Summary bool   `json:"include_http2_summary,omitempty" jsonschema:"Include HTTP/2 details (default: true)"`
	MaxBytes            int    `json:"max_bytes,omitempty" jsonschema:"Max body bytes for hashing"`
}

// FingerprintOutput is the output for powhttp_fingerprint.
type FingerprintOutput struct {
	Fingerprint *types.Fingerprint `json:"fingerprint"`
	Resource    *types.ResourceRef `json:"resource,omitempty"`
	Truncated   bool               `json:"truncated,omitempty"`
}

// ToolFingerprint generates a fingerprint for an entry.
func ToolFingerprint(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input FingerprintInput) (*sdkmcp.CallToolResult, FingerprintOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input FingerprintInput) (*sdkmcp.CallToolResult, FingerprintOutput, error) {
		if input.EntryID == "" {
			return nil, FingerprintOutput{}, ErrInvalidInput("entry_id is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		// Use defaults if not explicitly set to false
		includeTLS := input.IncludeTLSSummary
		includeHTTP2 := input.IncludeHTTP2Summary

		// Default to true if not explicitly set
		if !input.IncludeTLSSummary && !input.IncludeHTTP2Summary {
			includeTLS = true
			includeHTTP2 = true
		}

		opts := &types.FingerprintOptions{
			IncludeTLSSummary:   includeTLS,
			IncludeHTTP2Summary: includeHTTP2,
			MaxBytes:            input.MaxBytes,
		}
		if opts.MaxBytes <= 0 {
			opts.MaxBytes = d.Config.ToolMaxBytesDefault
		}

		fp, err := d.Fingerprint.Generate(ctx, sessionID, input.EntryID, opts)
		if err != nil {
			return nil, FingerprintOutput{}, WrapPowHTTPError(err)
		}

		return nil, FingerprintOutput{
			Fingerprint: fp,
		}, nil
	}
}
