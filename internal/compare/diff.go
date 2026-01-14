package compare

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// DiffEngine compares two HTTP entries and produces structured differences.
type DiffEngine struct {
	fingerprinter *FingerprintEngine
}

// NewDiffEngine creates a new DiffEngine.
func NewDiffEngine(fp *FingerprintEngine) *DiffEngine {
	return &DiffEngine{
		fingerprinter: fp,
	}
}

// Diff compares two entries and returns structured differences.
func (d *DiffEngine) Diff(ctx context.Context, req *types.DiffRequest) (*types.DiffResult, error) {
	// Apply defaults
	if req.SessionID == "" {
		req.SessionID = "active"
	}

	opts := req.Options
	if opts == nil {
		opts = &types.DiffOptions{
			CompareHeaderOrder:  true,
			CompareHeaderValues: true,
			CompareTLS:          true,
			CompareHTTP2:        true,
			IgnoreHeaders:       DefaultIgnoreHeaders,
			IgnoreQueryKeys:     DefaultIgnoreQueryKeys,
		}
	}

	// Apply default ignore lists if not set
	if opts.IgnoreHeaders == nil {
		opts.IgnoreHeaders = DefaultIgnoreHeaders
	}
	if opts.IgnoreQueryKeys == nil {
		opts.IgnoreQueryKeys = DefaultIgnoreQueryKeys
	}

	// Generate fingerprints
	fpOpts := &types.FingerprintOptions{
		IncludeTLSSummary:   opts.CompareTLS,
		IncludeHTTP2Summary: opts.CompareHTTP2,
		MaxBytes:            opts.MaxBytes,
	}

	baselineFP, err := d.fingerprinter.Generate(ctx, req.SessionID, req.BaselineEntryID, fpOpts)
	if err != nil {
		return nil, fmt.Errorf("generating baseline fingerprint: %w", err)
	}

	candidateFP, err := d.fingerprinter.Generate(ctx, req.SessionID, req.CandidateEntryID, fpOpts)
	if err != nil {
		return nil, fmt.Errorf("generating candidate fingerprint: %w", err)
	}

	result := &types.DiffResult{
		Baseline:  baselineFP.Entry,
		Candidate: candidateFP.Entry,
	}

	// Compare protocol versions
	if baselineFP.Entry.HTTPVersion != candidateFP.Entry.HTTPVersion {
		result.ImportantDiffs.Protocol = &types.ProtocolDiff{
			BaselineVersion:  baselineFP.Entry.HTTPVersion,
			CandidateVersion: candidateFP.Entry.HTTPVersion,
		}
	}

	// Compare TLS fingerprints
	if opts.CompareTLS {
		tlsDiff := diffTLS(baselineFP.TLSSummary, candidateFP.TLSSummary)
		if tlsDiff != nil {
			result.ImportantDiffs.TLS = tlsDiff
		}
	}

	// Compare HTTP/2 metadata
	if opts.CompareHTTP2 {
		h2Diff := diffHTTP2(baselineFP, candidateFP)
		if h2Diff != nil {
			result.ImportantDiffs.HTTP2 = h2Diff
		}
	}

	// Compare headers
	if opts.CompareHeaderValues {
		missing, extra, changed, ignored := diffHeaders(
			baselineFP.HeadersNormalized,
			candidateFP.HeadersNormalized,
			opts.IgnoreHeaders,
		)
		result.ImportantDiffs.HeadersMissing = missing
		result.ImportantDiffs.HeadersExtra = extra
		result.ImportantDiffs.HeadersValueChanged = changed
		result.NoisyDiffs.IgnoredHeaders = ignored
	}

	// Compare header order
	if opts.CompareHeaderOrder {
		orderDiff := diffHeaderOrder(baselineFP.HeadersOrdered, candidateFP.HeadersOrdered)
		if orderDiff != nil && (len(orderDiff.Moves) > 0 || !slices.Equal(orderDiff.BaselineOrder, orderDiff.CandidateOrder)) {
			result.ImportantDiffs.HeaderOrderChanges = orderDiff
		}
	}

	return result, nil
}

// diffHeaders compares header presence and values.
func diffHeaders(baseline, candidate map[string][]string, ignore []string) (missing, extra []string, changed []types.HeaderValueDiff, ignored []string) {
	ignoreSet := make(map[string]struct{}, len(ignore))
	for _, h := range ignore {
		ignoreSet[strings.ToLower(h)] = struct{}{}
	}

	// Find missing and changed headers
	for name, baselineValues := range baseline {
		if _, skip := ignoreSet[name]; skip {
			if _, inCandidate := candidate[name]; inCandidate {
				ignored = append(ignored, name)
			}
			continue
		}

		candidateValues, exists := candidate[name]
		if !exists {
			missing = append(missing, name)
			continue
		}

		// Compare values
		if !slices.Equal(baselineValues, candidateValues) {
			changed = append(changed, types.HeaderValueDiff{
				Name:      name,
				Baseline:  baselineValues,
				Candidate: candidateValues,
			})
		}
	}

	// Find extra headers
	for name := range candidate {
		if _, skip := ignoreSet[name]; skip {
			continue
		}
		if _, exists := baseline[name]; !exists {
			extra = append(extra, name)
		}
	}

	return missing, extra, changed, ignored
}

// diffHeaderOrder uses LCS to find order changes.
func diffHeaderOrder(baseline, candidate [][]string) *types.HeaderOrderDiff {
	// Extract header names (lowercase for comparison)
	baselineNames := extractHeaderNames(baseline)
	candidateNames := extractHeaderNames(candidate)

	// Compute LCS
	lcs := lcsHeaderOrder(baselineNames, candidateNames)
	lcsSet := make(map[string]struct{}, len(lcs))
	for _, h := range lcs {
		lcsSet[h] = struct{}{}
	}

	// Build position maps
	baselinePos := make(map[string]int, len(baselineNames))
	for i, name := range baselineNames {
		if _, exists := baselinePos[name]; !exists {
			baselinePos[name] = i
		}
	}

	candidatePos := make(map[string]int, len(candidateNames))
	for i, name := range candidateNames {
		if _, exists := candidatePos[name]; !exists {
			candidatePos[name] = i
		}
	}

	// Find moves (headers in both but not in LCS)
	var moves []types.OrderChange
	for name, bPos := range baselinePos {
		cPos, inCandidate := candidatePos[name]
		if !inCandidate {
			continue
		}
		if _, inLCS := lcsSet[name]; inLCS {
			continue
		}
		moves = append(moves, types.OrderChange{
			Header:       name,
			BaselinePos:  bPos,
			CandidatePos: cPos,
		})
	}

	return &types.HeaderOrderDiff{
		BaselineOrder:  baselineNames,
		CandidateOrder: candidateNames,
		Moves:          moves,
	}
}

// extractHeaderNames gets lowercase header names from ordered headers.
func extractHeaderNames(headers [][]string) []string {
	result := make([]string, 0, len(headers))
	for _, pair := range headers {
		if len(pair) >= 1 {
			// Skip pseudo-headers for order comparison
			if strings.HasPrefix(pair[0], ":") {
				continue
			}
			result = append(result, strings.ToLower(pair[0]))
		}
	}
	return result
}

// lcsHeaderOrder computes the longest common subsequence for header names.
func lcsHeaderOrder(a, b []string) []string {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return nil
	}

	// Build DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to find LCS
	lcsLen := dp[m][n]
	result := make([]string, lcsLen)
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcsLen--
			result[lcsLen] = a[i-1]
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}

// diffTLS compares TLS fingerprints.
func diffTLS(baseline, candidate *types.TLSFingerprint) *types.TLSDiff {
	if baseline == nil && candidate == nil {
		return nil
	}

	diff := &types.TLSDiff{}
	hasChanges := false

	baselineJA3, candidateJA3 := "", ""
	baselineJA4, candidateJA4 := "", ""
	baselineCipher, candidateCipher := "", ""
	baselineVersion, candidateVersion := "", ""

	if baseline != nil {
		baselineJA3 = baseline.JA3
		baselineJA4 = baseline.JA4
		baselineCipher = baseline.CipherSuite
		baselineVersion = baseline.TLSVersion
	}
	if candidate != nil {
		candidateJA3 = candidate.JA3
		candidateJA4 = candidate.JA4
		candidateCipher = candidate.CipherSuite
		candidateVersion = candidate.TLSVersion
	}

	// JA3 comparison (highest priority for anti-bot detection)
	if baselineJA3 != candidateJA3 && (baselineJA3 != "" || candidateJA3 != "") {
		diff.JA3Different = true
		diff.BaselineJA3 = baselineJA3
		diff.CandidateJA3 = candidateJA3
		hasChanges = true
	}

	// JA4 comparison (highest priority for anti-bot detection)
	if baselineJA4 != candidateJA4 && (baselineJA4 != "" || candidateJA4 != "") {
		diff.JA4Different = true
		diff.BaselineJA4 = baselineJA4
		diff.CandidateJA4 = candidateJA4
		hasChanges = true
	}

	// Cipher suite comparison
	if baselineCipher != candidateCipher && (baselineCipher != "" || candidateCipher != "") {
		diff.CipherDifferent = true
		hasChanges = true
	}

	// TLS version comparison
	if baselineVersion != candidateVersion && (baselineVersion != "" || candidateVersion != "") {
		diff.VersionDifferent = true
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return diff
}

// diffHTTP2 compares HTTP/2 metadata.
func diffHTTP2(baseline, candidate *types.Fingerprint) *types.HTTP2Diff {
	baselineH2 := baseline.HTTP2Summary
	candidateH2 := candidate.HTTP2Summary

	if baselineH2 == nil && candidateH2 == nil {
		return nil
	}

	diff := &types.HTTP2Diff{}
	hasChanges := false

	if baselineH2 != nil {
		diff.BaselineStreamID = baselineH2.StreamID
	}
	if candidateH2 != nil {
		diff.CandidateStreamID = candidateH2.StreamID
	}

	// Compare pseudo-headers
	if !pseudoHeadersEqual(baseline.HTTP2PseudoHeaders, candidate.HTTP2PseudoHeaders) {
		diff.PseudoHeadersDiff = true
		hasChanges = true
	}

	// Stream ID differences are informational, not necessarily important
	if diff.BaselineStreamID != diff.CandidateStreamID {
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return diff
}

// pseudoHeadersEqual compares two sets of pseudo-headers.
func pseudoHeadersEqual(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
