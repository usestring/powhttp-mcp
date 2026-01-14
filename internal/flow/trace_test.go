package flow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

func TestApplyTraceOptionsDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    *types.TraceOptions
		expected *types.TraceOptions
	}{
		{
			name:  "nil input returns defaults",
			input: nil,
			expected: &types.TraceOptions{
				TimeWindowMs: 120000,
				SamePIDOnly:  true,
				SameHostOnly: true,
			},
		},
		{
			name: "empty input uses defaults",
			input: &types.TraceOptions{
				TimeWindowMs: 0,
				SamePIDOnly:  false,
				SameHostOnly: false,
			},
			expected: &types.TraceOptions{
				TimeWindowMs: 120000,
				SamePIDOnly:  false,
				SameHostOnly: false,
			},
		},
		{
			name: "custom time window preserved",
			input: &types.TraceOptions{
				TimeWindowMs: 60000,
				SamePIDOnly:  true,
				SameHostOnly: true,
			},
			expected: &types.TraceOptions{
				TimeWindowMs: 60000,
				SamePIDOnly:  true,
				SameHostOnly: true,
			},
		},
		{
			name: "zero time window uses default",
			input: &types.TraceOptions{
				TimeWindowMs: 0,
				SamePIDOnly:  true,
				SameHostOnly: true,
			},
			expected: &types.TraceOptions{
				TimeWindowMs: 120000,
				SamePIDOnly:  true,
				SameHostOnly: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyTraceOptionsDefaults(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPruneGraph(t *testing.T) {
	tests := []struct {
		name         string
		graph        *types.FlowGraph
		seedEntryID  string
		limit        int
		expectedSize int
	}{
		{
			name: "graph smaller than limit unchanged",
			graph: &types.FlowGraph{
				Nodes: []types.FlowNode{
					{EntryID: "e1", Summary: &types.EntrySummary{EntryID: "e1"}},
					{EntryID: "e2", Summary: &types.EntrySummary{EntryID: "e2"}},
				},
				Edges: []types.FlowEdge{
					{From: "e1", To: "e2", Reason: types.EdgeReasonTemporal},
				},
			},
			seedEntryID:  "e1",
			limit:        5,
			expectedSize: 2,
		},
		{
			name: "prune to limit from seed",
			graph: &types.FlowGraph{
				Nodes: []types.FlowNode{
					{EntryID: "e1", Summary: &types.EntrySummary{EntryID: "e1"}},
					{EntryID: "e2", Summary: &types.EntrySummary{EntryID: "e2"}},
					{EntryID: "e3", Summary: &types.EntrySummary{EntryID: "e3"}},
					{EntryID: "e4", Summary: &types.EntrySummary{EntryID: "e4"}},
				},
				Edges: []types.FlowEdge{
					{From: "e1", To: "e2", Reason: types.EdgeReasonTemporal},
					{From: "e2", To: "e3", Reason: types.EdgeReasonTemporal},
					{From: "e3", To: "e4", Reason: types.EdgeReasonTemporal},
				},
			},
			seedEntryID:  "e1",
			limit:        2,
			expectedSize: 2,
		},
		{
			name: "BFS traversal order",
			graph: &types.FlowGraph{
				Nodes: []types.FlowNode{
					{EntryID: "seed", Summary: &types.EntrySummary{EntryID: "seed"}},
					{EntryID: "a1", Summary: &types.EntrySummary{EntryID: "a1"}},
					{EntryID: "a2", Summary: &types.EntrySummary{EntryID: "a2"}},
					{EntryID: "b1", Summary: &types.EntrySummary{EntryID: "b1"}},
					{EntryID: "b2", Summary: &types.EntrySummary{EntryID: "b2"}},
				},
				Edges: []types.FlowEdge{
					{From: "seed", To: "a1", Reason: types.EdgeReasonTemporal},
					{From: "seed", To: "a2", Reason: types.EdgeReasonTemporal},
					{From: "a1", To: "b1", Reason: types.EdgeReasonTemporal},
					{From: "a2", To: "b2", Reason: types.EdgeReasonTemporal},
				},
			},
			seedEntryID:  "seed",
			limit:        3,
			expectedSize: 3,
		},
		{
			name: "single node graph",
			graph: &types.FlowGraph{
				Nodes: []types.FlowNode{
					{EntryID: "e1", Summary: &types.EntrySummary{EntryID: "e1"}},
				},
				Edges: []types.FlowEdge{},
			},
			seedEntryID:  "e1",
			limit:        1,
			expectedSize: 1,
		},
		{
			name: "disconnected nodes not included",
			graph: &types.FlowGraph{
				Nodes: []types.FlowNode{
					{EntryID: "seed", Summary: &types.EntrySummary{EntryID: "seed"}},
					{EntryID: "connected", Summary: &types.EntrySummary{EntryID: "connected"}},
					{EntryID: "isolated", Summary: &types.EntrySummary{EntryID: "isolated"}},
				},
				Edges: []types.FlowEdge{
					{From: "seed", To: "connected", Reason: types.EdgeReasonTemporal},
				},
			},
			seedEntryID:  "seed",
			limit:        2,
			expectedSize: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pruneGraph(tt.graph, tt.seedEntryID, tt.limit)

			assert.Len(t, result.Nodes, tt.expectedSize)
			assert.LessOrEqual(t, len(result.Nodes), tt.limit, "should not exceed limit")

			// Verify seed is included
			found := false
			for _, node := range result.Nodes {
				if node.EntryID == tt.seedEntryID {
					found = true
					break
				}
			}
			assert.True(t, found, "seed node should be included")

			// Verify edges only connect nodes in the result
			nodeSet := make(map[string]bool)
			for _, node := range result.Nodes {
				nodeSet[node.EntryID] = true
			}

			for _, edge := range result.Edges {
				assert.True(t, nodeSet[edge.From], "edge from %q references missing node", edge.From)
				assert.True(t, nodeSet[edge.To], "edge to %q references missing node", edge.To)
			}
		})
	}
}

func TestPruneGraphEdgeCases(t *testing.T) {
	t.Run("empty graph", func(t *testing.T) {
		graph := &types.FlowGraph{Nodes: []types.FlowNode{}, Edges: []types.FlowEdge{}}
		result := pruneGraph(graph, "nonexistent", 10)
		assert.Empty(t, result.Nodes)
	})

	t.Run("limit zero", func(t *testing.T) {
		graph := &types.FlowGraph{
			Nodes: []types.FlowNode{{EntryID: "e1", Summary: &types.EntrySummary{EntryID: "e1"}}},
			Edges: []types.FlowEdge{},
		}
		result := pruneGraph(graph, "e1", 0)
		assert.Empty(t, result.Nodes)
	})

	t.Run("cyclic graph", func(t *testing.T) {
		graph := &types.FlowGraph{
			Nodes: []types.FlowNode{
				{EntryID: "e1", Summary: &types.EntrySummary{EntryID: "e1"}},
				{EntryID: "e2", Summary: &types.EntrySummary{EntryID: "e2"}},
				{EntryID: "e3", Summary: &types.EntrySummary{EntryID: "e3"}},
			},
			Edges: []types.FlowEdge{
				{From: "e1", To: "e2", Reason: types.EdgeReasonTemporal},
				{From: "e2", To: "e3", Reason: types.EdgeReasonTemporal},
				{From: "e3", To: "e1", Reason: types.EdgeReasonTemporal},
			},
		}
		result := pruneGraph(graph, "e1", 2)
		assert.Len(t, result.Nodes, 2)
	})
}

// BuildEdgesTestSuite uses testify/suite for complex edge building tests
type BuildEdgesTestSuite struct {
	suite.Suite
	engine *FlowEngine
}

func (s *BuildEdgesTestSuite) SetupTest() {
	s.engine = &FlowEngine{}
}

func (s *BuildEdgesTestSuite) TestEmptyEntries() {
	edges := s.engine.buildEdges([]*indexer.EntryMeta{}, nil)
	s.Empty(edges)
}

func (s *BuildEdgesTestSuite) TestSingleEntry() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000},
	}
	edges := s.engine.buildEdges(entries, nil)
	s.Empty(edges)
}

func (s *BuildEdgesTestSuite) TestTLSConnectionEdges() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000, TLSConnectionID: "tls-1"},
		{EntryID: "e2", TsMs: 2000, TLSConnectionID: "tls-1"},
		{EntryID: "e3", TsMs: 3000, TLSConnectionID: "tls-1"},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.Len(edges, 2)
	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonSameTLS)
	s.assertEdgeExists(edges, "e2", "e3", types.EdgeReasonSameTLS)
}

func (s *BuildEdgesTestSuite) TestHTTP2ConnectionEdges() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000, H2ConnectionID: "h2-1"},
		{EntryID: "e2", TsMs: 2000, H2ConnectionID: "h2-1"},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.Len(edges, 1)
	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonSameH2)
}

func (s *BuildEdgesTestSuite) TestSameAuthorizationHeaderEdges() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000, AuthHeader: "Bearer token123"},
		{EntryID: "e2", TsMs: 2000, AuthHeader: "Bearer token123"},
		{EntryID: "e3", TsMs: 3000, AuthHeader: "Bearer token123"},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.GreaterOrEqual(len(edges), 2)
	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonSameAuth)
	s.assertEdgeExists(edges, "e2", "e3", types.EdgeReasonSameAuth)
}

func (s *BuildEdgesTestSuite) TestCookieSetUseEdges() {
	entries := []*indexer.EntryMeta{
		{
			EntryID:    "e1",
			TsMs:       1000,
			Host:       "api.example.com",
			SetCookies: map[string]string{"session_id": "abc123"},
		},
		{
			EntryID: "e2",
			TsMs:    2000,
			Host:    "api.example.com",
			Cookies: map[string]string{"session_id": "abc123"},
		},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonSessionCookieOrigin)
}

func (s *BuildEdgesTestSuite) TestCookieSameHostOnly() {
	entries := []*indexer.EntryMeta{
		{
			EntryID:    "e1",
			TsMs:       1000,
			Host:       "api1.example.com",
			SetCookies: map[string]string{"session_id": "abc123"},
		},
		{
			EntryID: "e2",
			TsMs:    2000,
			Host:    "api2.example.com",
			Cookies: map[string]string{"session_id": "abc123"},
		},
	}

	edges := s.engine.buildEdges(entries, nil)

	// Should not create cookie origin edge across different hosts
	for _, edge := range edges {
		s.NotEqual(types.EdgeReasonSessionCookieOrigin, edge.Reason,
			"should not create cookie origin edge across different hosts")
	}
}

func (s *BuildEdgesTestSuite) TestTemporalEdges() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000},
		{EntryID: "e2", TsMs: 2000},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.Len(edges, 1)
	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonTemporal)
}

func (s *BuildEdgesTestSuite) TestTLSOverridesTemporal() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e1", TsMs: 1000, TLSConnectionID: "tls-1"},
		{EntryID: "e2", TsMs: 2000, TLSConnectionID: "tls-1"},
	}

	edges := s.engine.buildEdges(entries, nil)

	s.Len(edges, 1)
	s.Equal(types.EdgeReasonSameTLS, edges[0].Reason)
}

func (s *BuildEdgesTestSuite) TestNoDuplicateEdges() {
	entries := []*indexer.EntryMeta{
		{
			EntryID:         "e1",
			TsMs:            1000,
			TLSConnectionID: "tls-1",
			H2ConnectionID:  "h2-1",
		},
		{
			EntryID:         "e2",
			TsMs:            2000,
			TLSConnectionID: "tls-1",
			H2ConnectionID:  "h2-1",
		},
	}

	edges := s.engine.buildEdges(entries, nil)

	// Should have both same_tls and same_h2, but no duplicates
	edgesByReason := make(map[string]int)
	for _, edge := range edges {
		edgesByReason[edge.Reason]++
	}

	s.Equal(1, edgesByReason[types.EdgeReasonSameTLS])
	s.Equal(1, edgesByReason[types.EdgeReasonSameH2])
}

func (s *BuildEdgesTestSuite) TestTimestampOrdering() {
	entries := []*indexer.EntryMeta{
		{EntryID: "e3", TsMs: 3000, TLSConnectionID: "tls-1"},
		{EntryID: "e1", TsMs: 1000, TLSConnectionID: "tls-1"},
		{EntryID: "e2", TsMs: 2000, TLSConnectionID: "tls-1"},
	}

	edges := s.engine.buildEdges(entries, nil)

	// Should be sorted by timestamp: e1->e2->e3
	s.assertEdgeExists(edges, "e1", "e2", types.EdgeReasonSameTLS)
	s.assertEdgeExists(edges, "e2", "e3", types.EdgeReasonSameTLS)
}

func (s *BuildEdgesTestSuite) TestComplexLoginFlow() {
	// Simulate: login request sets cookie, then API calls use it
	entries := []*indexer.EntryMeta{
		{
			EntryID:         "login",
			TsMs:            1000,
			Host:            "api.example.com",
			TLSConnectionID: "tls-1",
			SetCookies:      map[string]string{"session_id": "abc123"},
		},
		{
			EntryID:         "get-profile",
			TsMs:            2000,
			Host:            "api.example.com",
			TLSConnectionID: "tls-1",
			Cookies:         map[string]string{"session_id": "abc123"},
			AuthHeader:      "Bearer token123",
		},
		{
			EntryID:         "get-data",
			TsMs:            3000,
			Host:            "api.example.com",
			TLSConnectionID: "tls-1",
			Cookies:         map[string]string{"session_id": "abc123"},
			AuthHeader:      "Bearer token123",
		},
	}

	edges := s.engine.buildEdges(entries, nil)

	// Count edge types
	edgeTypes := make(map[string]int)
	for _, edge := range edges {
		edgeTypes[edge.Reason]++
	}

	// Verify expected edge types
	s.Equal(2, edgeTypes[types.EdgeReasonSameTLS], "should have 2 TLS edges")
	s.Equal(1, edgeTypes[types.EdgeReasonSameAuth], "should have 1 auth edge")
	s.Equal(1, edgeTypes[types.EdgeReasonSameSessionCookie], "should have 1 session cookie edge")
	s.Equal(2, edgeTypes[types.EdgeReasonSessionCookieOrigin], "should have 2 cookie origin edges")
	s.Zero(edgeTypes[types.EdgeReasonTemporal], "should not have temporal edges")
}

func (s *BuildEdgesTestSuite) TestEarliestCookieSetterWins() {
	entries := []*indexer.EntryMeta{
		{
			EntryID:    "e1",
			TsMs:       1000,
			Host:       "api.example.com",
			SetCookies: map[string]string{"session_id": "abc123"},
		},
		{
			EntryID:    "e2",
			TsMs:       1500,
			Host:       "api.example.com",
			SetCookies: map[string]string{"session_id": "abc123"},
		},
		{
			EntryID: "e3",
			TsMs:    2000,
			Host:    "api.example.com",
			Cookies: map[string]string{"session_id": "abc123"},
		},
	}

	edges := s.engine.buildEdges(entries, nil)

	// Should only have one cookie origin edge from earliest setter (e1)
	cookieOriginCount := 0
	for _, edge := range edges {
		if edge.Reason == types.EdgeReasonSessionCookieOrigin {
			cookieOriginCount++
			s.Equal("e1", edge.From, "cookie origin should be from earliest setter")
		}
	}

	s.Equal(1, cookieOriginCount)
}

// Helper assertion
func (s *BuildEdgesTestSuite) assertEdgeExists(edges []types.FlowEdge, from, to, reason string) {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Reason == reason {
			return
		}
	}
	s.Failf("edge not found", "expected edge %s -> %s (%s)", from, to, reason)
}

func TestBuildEdgesTestSuite(t *testing.T) {
	suite.Run(t, new(BuildEdgesTestSuite))
}
