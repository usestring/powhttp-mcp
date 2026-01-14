package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestDiffHeaders(t *testing.T) {
	tests := []struct {
		name        string
		baseline    map[string][]string
		candidate   map[string][]string
		ignore      []string
		wantMissing []string
		wantExtra   []string
		wantChanged []types.HeaderValueDiff
		wantIgnored []string
	}{
		{
			name: "no differences",
			baseline: map[string][]string{
				"content-type": {"application/json"},
				"accept":       {"*/*"},
			},
			candidate: map[string][]string{
				"content-type": {"application/json"},
				"accept":       {"*/*"},
			},
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{},
		},
		{
			name: "missing header",
			baseline: map[string][]string{
				"content-type":  {"application/json"},
				"authorization": {"Bearer token"},
			},
			candidate: map[string][]string{
				"content-type": {"application/json"},
			},
			wantMissing: []string{"authorization"},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{},
		},
		{
			name: "extra header",
			baseline: map[string][]string{
				"content-type": {"application/json"},
			},
			candidate: map[string][]string{
				"content-type": {"application/json"},
				"x-custom":     {"value"},
			},
			wantMissing: []string{},
			wantExtra:   []string{"x-custom"},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{},
		},
		{
			name: "changed header value",
			baseline: map[string][]string{
				"user-agent": {"Mozilla/5.0"},
			},
			candidate: map[string][]string{
				"user-agent": {"Python/3.9"},
			},
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{
				{
					Name:      "user-agent",
					Baseline:  []string{"Mozilla/5.0"},
					Candidate: []string{"Python/3.9"},
				},
			},
			wantIgnored: []string{},
		},
		{
			name: "ignored headers",
			baseline: map[string][]string{
				"date":         {"Mon, 01 Jan 2024 00:00:00 GMT"},
				"content-type": {"application/json"},
			},
			candidate: map[string][]string{
				"date":         {"Tue, 02 Jan 2024 00:00:00 GMT"},
				"content-type": {"application/json"},
			},
			ignore:      []string{"date"},
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{"date"},
		},
		{
			name: "multiple value header",
			baseline: map[string][]string{
				"accept-encoding": {"gzip", "deflate"},
			},
			candidate: map[string][]string{
				"accept-encoding": {"gzip"},
			},
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{
				{
					Name:      "accept-encoding",
					Baseline:  []string{"gzip", "deflate"},
					Candidate: []string{"gzip"},
				},
			},
			wantIgnored: []string{},
		},
		{
			name: "case insensitive ignore",
			baseline: map[string][]string{
				"x-request-id": {"abc123"},
			},
			candidate: map[string][]string{
				"x-request-id": {"def456"},
			},
			ignore:      []string{"X-Request-ID"},
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{"x-request-id"},
		},
		{
			name: "missing ignored header not reported",
			baseline: map[string][]string{
				"date": {"Mon, 01 Jan 2024 00:00:00 GMT"},
			},
			candidate: map[string][]string{},
			ignore:    []string{"date"},
			// When a header is ignored, it's not reported as missing
			wantMissing: []string{},
			wantExtra:   []string{},
			wantChanged: []types.HeaderValueDiff{},
			wantIgnored: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing, extra, changed, ignored := diffHeaders(tt.baseline, tt.candidate, tt.ignore)

			assert.ElementsMatch(t, tt.wantMissing, missing, "missing headers")
			assert.ElementsMatch(t, tt.wantExtra, extra, "extra headers")
			assert.ElementsMatch(t, tt.wantIgnored, ignored, "ignored headers")

			// For changed headers, compare individually
			if len(tt.wantChanged) > 0 {
				require.Len(t, changed, len(tt.wantChanged), "changed headers count")
				for _, want := range tt.wantChanged {
					found := false
					for _, got := range changed {
						if got.Name == want.Name {
							assert.Equal(t, want.Baseline, got.Baseline, "baseline values for %s", want.Name)
							assert.Equal(t, want.Candidate, got.Candidate, "candidate values for %s", want.Name)
							found = true
							break
						}
					}
					assert.True(t, found, "expected changed header %s not found", want.Name)
				}
			}
		})
	}
}

func TestLcsHeaderOrder(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "identical sequences",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "completely different",
			a:        []string{"a", "b", "c"},
			b:        []string{"x", "y", "z"},
			expected: []string{},
		},
		{
			name:     "partial overlap",
			a:        []string{"a", "b", "c", "d"},
			b:        []string{"b", "d"},
			expected: []string{"b", "d"},
		},
		{
			name:     "reordered with common subsequence",
			a:        []string{"a", "b", "c", "d"},
			b:        []string{"d", "c", "b", "a"},
			expected: []string{"d"},
		},
		{
			name:     "one empty",
			a:        []string{"a", "b", "c"},
			b:        []string{},
			expected: nil,
		},
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: nil,
		},
		{
			name:     "insertion in middle",
			a:        []string{"a", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "c"},
		},
		{
			name:     "deletion from middle",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "c"},
			expected: []string{"a", "c"},
		},
		{
			name:     "multiple common subsequences",
			a:        []string{"x", "a", "b", "y", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lcsHeaderOrder(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiffHeaderOrder(t *testing.T) {
	tests := []struct {
		name            string
		baseline        [][]string
		candidate       [][]string
		expectMoves     bool
		expectOrderDiff bool
	}{
		{
			name: "identical order",
			baseline: [][]string{
				{"content-type", "application/json"},
				{"accept", "*/*"},
			},
			candidate: [][]string{
				{"content-type", "application/json"},
				{"accept", "*/*"},
			},
		},
		{
			name: "reordered headers",
			baseline: [][]string{
				{"content-type", "application/json"},
				{"accept", "*/*"},
			},
			candidate: [][]string{
				{"accept", "*/*"},
				{"content-type", "application/json"},
			},
			expectMoves:     true,
			expectOrderDiff: true,
		},
		{
			name: "pseudo-headers skipped",
			baseline: [][]string{
				{":method", "GET"},
				{":path", "/api"},
				{"accept", "*/*"},
			},
			candidate: [][]string{
				{":method", "GET"},
				{":path", "/api"},
				{"accept", "*/*"},
			},
		},
		{
			name:      "empty headers",
			baseline:  [][]string{},
			candidate: [][]string{},
		},
		{
			name: "header present in one missing in other",
			baseline: [][]string{
				{"content-type", "application/json"},
				{"authorization", "Bearer token"},
			},
			candidate: [][]string{
				{"content-type", "application/json"},
			},
			expectOrderDiff: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diffHeaderOrder(tt.baseline, tt.candidate)

			require.NotNil(t, result)

			if tt.expectMoves {
				assert.NotEmpty(t, result.Moves, "expected moves but got none")
			} else if !tt.expectOrderDiff {
				assert.Empty(t, result.Moves, "expected no moves")
			}

			if tt.expectOrderDiff {
				assert.NotEqual(t, result.BaselineOrder, result.CandidateOrder,
					"expected order difference")
			}
		})
	}
}

func TestExtractHeaderNames(t *testing.T) {
	tests := []struct {
		name     string
		headers  [][]string
		expected []string
	}{
		{
			name: "normal headers",
			headers: [][]string{
				{"Content-Type", "application/json"},
				{"Accept", "*/*"},
			},
			expected: []string{"content-type", "accept"},
		},
		{
			name: "skip pseudo-headers",
			headers: [][]string{
				{":method", "GET"},
				{":path", "/api"},
				{"accept", "*/*"},
			},
			expected: []string{"accept"},
		},
		{
			name:     "empty headers",
			headers:  [][]string{},
			expected: []string{},
		},
		{
			name: "mixed case lowercased",
			headers: [][]string{
				{"Content-Type", "text/html"},
				{"USER-AGENT", "test"},
			},
			expected: []string{"content-type", "user-agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHeaderNames(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiffTLS(t *testing.T) {
	tests := []struct {
		name       string
		baseline   *types.TLSFingerprint
		candidate  *types.TLSFingerprint
		wantNil    bool
		assertions func(t *testing.T, diff *types.TLSDiff)
	}{
		{
			name:      "both nil",
			baseline:  nil,
			candidate: nil,
			wantNil:   true,
		},
		{
			name: "identical fingerprints",
			baseline: &types.TLSFingerprint{
				JA3:         "abc123",
				JA4:         "def456",
				CipherSuite: "TLS_AES_128_GCM_SHA256",
				TLSVersion:  "TLS 1.3",
			},
			candidate: &types.TLSFingerprint{
				JA3:         "abc123",
				JA4:         "def456",
				CipherSuite: "TLS_AES_128_GCM_SHA256",
				TLSVersion:  "TLS 1.3",
			},
			wantNil: true,
		},
		{
			name:      "JA3 different",
			baseline:  &types.TLSFingerprint{JA3: "abc123"},
			candidate: &types.TLSFingerprint{JA3: "xyz789"},
			assertions: func(t *testing.T, diff *types.TLSDiff) {
				assert.True(t, diff.JA3Different)
				assert.Equal(t, "abc123", diff.BaselineJA3)
				assert.Equal(t, "xyz789", diff.CandidateJA3)
			},
		},
		{
			name:      "JA4 different",
			baseline:  &types.TLSFingerprint{JA4: "def456"},
			candidate: &types.TLSFingerprint{JA4: "uvw012"},
			assertions: func(t *testing.T, diff *types.TLSDiff) {
				assert.True(t, diff.JA4Different)
			},
		},
		{
			name:      "cipher different",
			baseline:  &types.TLSFingerprint{CipherSuite: "TLS_AES_128_GCM_SHA256"},
			candidate: &types.TLSFingerprint{CipherSuite: "TLS_CHACHA20_POLY1305_SHA256"},
			assertions: func(t *testing.T, diff *types.TLSDiff) {
				assert.True(t, diff.CipherDifferent)
			},
		},
		{
			name:      "baseline missing JA3",
			baseline:  &types.TLSFingerprint{JA3: ""},
			candidate: &types.TLSFingerprint{JA3: "abc123"},
			assertions: func(t *testing.T, diff *types.TLSDiff) {
				assert.True(t, diff.JA3Different)
			},
		},
		{
			name: "multiple differences",
			baseline: &types.TLSFingerprint{
				JA3:         "abc",
				JA4:         "def",
				CipherSuite: "cipher1",
				TLSVersion:  "TLS 1.2",
			},
			candidate: &types.TLSFingerprint{
				JA3:         "xyz",
				JA4:         "uvw",
				CipherSuite: "cipher2",
				TLSVersion:  "TLS 1.3",
			},
			assertions: func(t *testing.T, diff *types.TLSDiff) {
				assert.True(t, diff.JA3Different)
				assert.True(t, diff.JA4Different)
				assert.True(t, diff.CipherDifferent)
				assert.True(t, diff.VersionDifferent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diffTLS(tt.baseline, tt.candidate)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			if tt.assertions != nil {
				tt.assertions(t, result)
			}
		})
	}
}

func TestPseudoHeadersEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        [][]string
		b        [][]string
		expected bool
	}{
		{
			name: "identical",
			a: [][]string{
				{":method", "GET"},
				{":path", "/api"},
			},
			b: [][]string{
				{":method", "GET"},
				{":path", "/api"},
			},
			expected: true,
		},
		{
			name: "different values",
			a: [][]string{
				{":method", "GET"},
			},
			b: [][]string{
				{":method", "POST"},
			},
			expected: false,
		},
		{
			name: "different lengths",
			a: [][]string{
				{":method", "GET"},
				{":path", "/api"},
			},
			b: [][]string{
				{":method", "GET"},
			},
			expected: false,
		},
		{
			name:     "both empty",
			a:        [][]string{},
			b:        [][]string{},
			expected: true,
		},
		{
			name: "different order",
			a: [][]string{
				{":method", "GET"},
				{":path", "/api"},
			},
			b: [][]string{
				{":path", "/api"},
				{":method", "GET"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pseudoHeadersEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
