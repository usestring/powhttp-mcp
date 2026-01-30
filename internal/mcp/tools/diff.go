package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// DiffEntriesInput is the input for powhttp_diff_entries.
type DiffEntriesInput struct {
	SessionID        string       `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
	BaselineEntryID  string       `json:"baseline_entry_id" jsonschema:"required,Baseline (browser) entry ID"`
	CandidateEntryID string       `json:"candidate_entry_id" jsonschema:"required,Candidate (program) entry ID"`
	Options          *DiffOptions `json:"options,omitempty" jsonschema:"Diff options"`
	MaxBytes         int          `json:"max_bytes,omitempty" jsonschema:"Max body bytes for comparison"`
}

// DiffOptions controls diff behavior.
type DiffOptions struct {
	CompareHeaderOrder  bool     `json:"compare_header_order,omitempty" jsonschema:"Compare header order (default: true)"`
	CompareHeaderValues bool     `json:"compare_header_values,omitempty" jsonschema:"Compare header values (default: true)"`
	CompareTLS          bool     `json:"compare_tls,omitempty" jsonschema:"Compare TLS fingerprints (default: true)"`
	CompareHTTP2        bool     `json:"compare_http2,omitempty" jsonschema:"Compare HTTP/2 metadata (default: true)"`
	IgnoreHeaders       []string `json:"ignore_headers,omitempty" jsonschema:"Headers to ignore"`
	IgnoreQueryKeys     []string `json:"ignore_query_keys,omitempty" jsonschema:"Query keys to ignore"`
}

// DiffEntriesOutput is the output for powhttp_diff_entries.
type DiffEntriesOutput struct {
	Diff     *types.DiffResult  `json:"diff"`
	Severity string             `json:"severity,omitempty"`
	Resource *types.ResourceRef `json:"resource,omitempty"`
}

// ToolDiffEntries compares two entries.
func ToolDiffEntries(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input DiffEntriesInput) (*sdkmcp.CallToolResult, DiffEntriesOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input DiffEntriesInput) (*sdkmcp.CallToolResult, DiffEntriesOutput, error) {
		if input.BaselineEntryID == "" {
			return nil, DiffEntriesOutput{}, ErrInvalidInput("baseline_entry_id is required")
		}
		if input.CandidateEntryID == "" {
			return nil, DiffEntriesOutput{}, ErrInvalidInput("candidate_entry_id is required")
		}

		sessionID := input.SessionID
		if sessionID == "" {
			sessionID = "active"
		}

		diffReq := &types.DiffRequest{
			BaselineEntryID:  input.BaselineEntryID,
			CandidateEntryID: input.CandidateEntryID,
			SessionID:        sessionID,
		}

		if input.Options != nil {
			diffReq.Options = &types.DiffOptions{
				CompareHeaderOrder:  input.Options.CompareHeaderOrder,
				CompareHeaderValues: input.Options.CompareHeaderValues,
				CompareTLS:          input.Options.CompareTLS,
				CompareHTTP2:        input.Options.CompareHTTP2,
				IgnoreHeaders:       input.Options.IgnoreHeaders,
				IgnoreQueryKeys:     input.Options.IgnoreQueryKeys,
				MaxBytes:            input.MaxBytes,
			}
		}

		result, err := d.Diff.Diff(ctx, diffReq)
		if err != nil {
			return nil, DiffEntriesOutput{}, WrapPowHTTPError(err)
		}

		return nil, DiffEntriesOutput{
			Diff:     result,
			Severity: computeDiffSeverity(result),
			Resource: &types.ResourceRef{
				URI:  "powhttp://diff/" + input.BaselineEntryID + "/" + input.CandidateEntryID,
				MIME: MimeJSON,
				Hint: "Fetch for full raw comparison data",
			},
		}, nil
	}
}

// computeDiffSeverity computes severity from diff result.
// "high": JA4 TLS fingerprint mismatch or protocol mismatch or many missing headers.
// "medium": Header order significantly different or a few missing/extra headers.
// "low": Only noisy diffs.
// "none": No meaningful differences.
func computeDiffSeverity(result *types.DiffResult) string {
	if result == nil {
		return "none"
	}

	imp := result.ImportantDiffs

	// High: JA4 TLS fingerprint mismatch or protocol mismatch
	if imp.TLS != nil && imp.TLS.JA4Different {
		return "high"
	}
	if imp.Protocol != nil {
		return "high"
	}

	// High: many missing headers (3+)
	missingCount := len(imp.HeadersMissing) + len(imp.HeadersExtra)
	if missingCount >= 3 {
		return "high"
	}

	// Medium: header order changes or some missing/extra headers
	if imp.HeaderOrderChanges != nil || missingCount > 0 || len(imp.HeadersValueChanged) > 0 {
		return "medium"
	}

	// Medium: HTTP/2 diffs
	if imp.HTTP2 != nil {
		return "medium"
	}

	// Low: only noisy diffs
	noisy := result.NoisyDiffs
	if len(noisy.IgnoredHeaders) > 0 || len(noisy.QueryKeyDiffs) > 0 {
		return "low"
	}

	return "none"
}
