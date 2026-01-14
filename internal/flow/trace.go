// Package flow provides request flow graph construction.
package flow

import (
	"context"
	"fmt"
	"sort"

	"github.com/RoaringBitmap/roaring/v2"

	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// FlowEngine reconstructs request flow graphs.
type FlowEngine struct {
	indexer *indexer.Indexer
	config  *config.Config
}

// NewFlowEngine creates a new FlowEngine.
func NewFlowEngine(idx *indexer.Indexer, cfg *config.Config) *FlowEngine {
	return &FlowEngine{
		indexer: idx,
		config:  cfg,
	}
}

// Trace reconstructs a flow graph around a seed entry.
func (f *FlowEngine) Trace(ctx context.Context, req *types.TraceRequest) (*types.FlowGraph, error) {
	// Ensure index is fresh
	if err := f.indexer.RefreshIfStale(ctx, req.SessionID); err != nil {
		return nil, fmt.Errorf("refreshing index: %w", err)
	}

	// Get seed entry metadata
	seed := f.indexer.GetMetaByEntryID(req.SeedEntryID)
	if seed == nil {
		return nil, fmt.Errorf("seed entry not found: %s", req.SeedEntryID)
	}

	// Apply defaults
	opts := applyTraceOptionsDefaults(req.Options)
	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 50
	}
	if maxDepth > 500 {
		maxDepth = 500
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	// Find related entries
	relatedDocIDs := f.findRelatedEntries(seed, opts)

	// Collect metadata for related entries
	entries := make([]*indexer.EntryMeta, 0, relatedDocIDs.GetCardinality())
	iter := relatedDocIDs.Iterator()
	for iter.HasNext() {
		docID := iter.Next()
		meta := f.indexer.GetMeta(docID)
		if meta != nil {
			entries = append(entries, meta)
		}
	}

	// Sort by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TsMs < entries[j].TsMs
	})

	// Build edges
	edges := f.buildEdges(entries, seed)

	// Build initial graph
	graph := &types.FlowGraph{
		Nodes: make([]types.FlowNode, 0, len(entries)),
		Edges: edges,
	}

	for _, meta := range entries {
		graph.Nodes = append(graph.Nodes, types.FlowNode{
			EntryID: meta.EntryID,
			Summary: meta.ToSummary(),
		})
	}

	// Prune graph if needed
	if len(graph.Nodes) > limit {
		graph = pruneGraph(graph, req.SeedEntryID, limit)
	}

	return graph, nil
}

// applyTraceOptionsDefaults applies default values to TraceOptions.
func applyTraceOptionsDefaults(opts *types.TraceOptions) *types.TraceOptions {
	result := &types.TraceOptions{
		TimeWindowMs: 120000, // 2 minutes
		SamePIDOnly:  true,
		SameHostOnly: true,
	}

	if opts == nil {
		return result
	}

	if opts.TimeWindowMs > 0 {
		result.TimeWindowMs = opts.TimeWindowMs
	}
	result.SamePIDOnly = opts.SamePIDOnly
	result.SameHostOnly = opts.SameHostOnly

	return result
}

// findRelatedEntries finds entries related to the seed using various heuristics.
func (f *FlowEngine) findRelatedEntries(seed *indexer.EntryMeta, opts *types.TraceOptions) *roaring.Bitmap {
	result := roaring.New()

	// Always include the seed
	result.Add(seed.DocID)

	// Find entries by TLS connection
	if seed.TLSConnectionID != "" {
		if bm := f.indexer.GetBitmapForTLSConnection(seed.TLSConnectionID); bm != nil {
			result.Or(bm)
		}
	}

	// Find entries by HTTP/2 connection
	if seed.H2ConnectionID != "" {
		if bm := f.indexer.GetBitmapForH2Connection(seed.H2ConnectionID); bm != nil {
			result.Or(bm)
		}
	}

	// Find temporal neighbors (entries within time window)
	timeWindow := opts.TimeWindowMs
	sinceMs := seed.TsMs - timeWindow/2
	untilMs := seed.TsMs + timeWindow/2

	// Start with all docs, then filter
	allDocs := f.indexer.AllDocIDs()
	iter := allDocs.Iterator()
	for iter.HasNext() {
		docID := iter.Next()
		meta := f.indexer.GetMeta(docID)
		if meta == nil {
			continue
		}

		// Time filter
		if meta.TsMs < sinceMs || meta.TsMs > untilMs {
			continue
		}

		// PID filter
		if opts.SamePIDOnly && meta.PID != seed.PID {
			continue
		}

		// Host filter
		if opts.SameHostOnly && meta.Host != seed.Host {
			continue
		}

		result.Add(docID)
	}

	return result
}

// buildEdges determines relationships between entries.
func (f *FlowEngine) buildEdges(entries []*indexer.EntryMeta, seed *indexer.EntryMeta) []types.FlowEdge {
	edges := make([]types.FlowEdge, 0)
	edgeSet := make(map[string]bool) // Prevent duplicate edges

	addEdge := func(from, to, reason string) {
		key := from + "->" + to + ":" + reason
		if edgeSet[key] {
			return
		}
		edgeSet[key] = true
		edges = append(edges, types.FlowEdge{
			From:   from,
			To:     to,
			Reason: reason,
		})
	}

	// Build index for quick lookup
	byTLS := make(map[string][]*indexer.EntryMeta)
	byH2 := make(map[string][]*indexer.EntryMeta)
	byAuth := make(map[string][]*indexer.EntryMeta)
	byAPIKey := make(map[string][]*indexer.EntryMeta)                  // key: "header:value"
	byCookie := make(map[string][]*indexer.EntryMeta)                  // key: "name:value"
	setCookieEntries := make(map[string]map[string]*indexer.EntryMeta) // host -> cookie name -> entry that set it

	for _, e := range entries {
		if e.TLSConnectionID != "" {
			byTLS[e.TLSConnectionID] = append(byTLS[e.TLSConnectionID], e)
		}
		if e.H2ConnectionID != "" {
			byH2[e.H2ConnectionID] = append(byH2[e.H2ConnectionID], e)
		}
		if e.AuthHeader != "" {
			byAuth[e.AuthHeader] = append(byAuth[e.AuthHeader], e)
		}
		for name, value := range e.APIKeys {
			key := name + ":" + value
			byAPIKey[key] = append(byAPIKey[key], e)
		}
		for name, value := range e.Cookies {
			key := name + ":" + value
			byCookie[key] = append(byCookie[key], e)
		}
		// Track Set-Cookie entries by host for cookie_set_use edges
		for name := range e.SetCookies {
			if setCookieEntries[e.Host] == nil {
				setCookieEntries[e.Host] = make(map[string]*indexer.EntryMeta)
			}
			// Store the earliest entry that set this cookie
			if existing := setCookieEntries[e.Host][name]; existing == nil || e.TsMs < existing.TsMs {
				setCookieEntries[e.Host][name] = e
			}
		}
	}

	// Add TLS connection edges
	for _, group := range byTLS {
		if len(group) < 2 {
			continue
		}
		// Sort by timestamp within group
		sort.Slice(group, func(i, j int) bool {
			return group[i].TsMs < group[j].TsMs
		})
		// Connect sequential entries
		for i := 0; i < len(group)-1; i++ {
			addEdge(group[i].EntryID, group[i+1].EntryID, types.EdgeReasonSameTLS)
		}
	}

	// Add H2 connection edges
	for _, group := range byH2 {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].TsMs < group[j].TsMs
		})
		for i := 0; i < len(group)-1; i++ {
			addEdge(group[i].EntryID, group[i+1].EntryID, types.EdgeReasonSameH2)
		}
	}

	// Add same Authorization header edges
	for _, group := range byAuth {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].TsMs < group[j].TsMs
		})
		for i := 0; i < len(group)-1; i++ {
			addEdge(group[i].EntryID, group[i+1].EntryID, types.EdgeReasonSameAuth)
		}
	}

	// Add same API key edges
	for _, group := range byAPIKey {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].TsMs < group[j].TsMs
		})
		for i := 0; i < len(group)-1; i++ {
			addEdge(group[i].EntryID, group[i+1].EntryID, types.EdgeReasonSameAPIKey)
		}
	}

	// Add same session cookie edges
	for _, group := range byCookie {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].TsMs < group[j].TsMs
		})
		for i := 0; i < len(group)-1; i++ {
			addEdge(group[i].EntryID, group[i+1].EntryID, types.EdgeReasonSameSessionCookie)
		}
	}

	// Add cookie set/use edges (same host only)
	for _, e := range entries {
		if e.Cookies == nil {
			continue
		}
		hostSetCookies := setCookieEntries[e.Host]
		if hostSetCookies == nil {
			continue
		}
		for cookieName := range e.Cookies {
			if setter := hostSetCookies[cookieName]; setter != nil && setter.EntryID != e.EntryID {
				// Only create edge if setter came before this entry
				if setter.TsMs < e.TsMs {
					addEdge(setter.EntryID, e.EntryID, types.EdgeReasonSessionCookieOrigin)
				}
			}
		}
	}

	// Add temporal edges for sequential entries
	for i := 0; i < len(entries)-1; i++ {
		// Only add temporal edge if no stronger relationship exists
		curr := entries[i]
		next := entries[i+1]

		// Check if already connected by TLS or H2
		hasTLS := curr.TLSConnectionID != "" && curr.TLSConnectionID == next.TLSConnectionID
		hasH2 := curr.H2ConnectionID != "" && curr.H2ConnectionID == next.H2ConnectionID

		if !hasTLS && !hasH2 {
			addEdge(curr.EntryID, next.EntryID, types.EdgeReasonTemporal)
		}
	}

	return edges
}

// pruneGraph limits graph to max nodes while preserving connectivity from seed.
func pruneGraph(graph *types.FlowGraph, seedEntryID string, limit int) *types.FlowGraph {
	if len(graph.Nodes) <= limit {
		return graph
	}

	// Build adjacency list
	adjacent := make(map[string][]string)
	for _, edge := range graph.Edges {
		adjacent[edge.From] = append(adjacent[edge.From], edge.To)
		adjacent[edge.To] = append(adjacent[edge.To], edge.From)
	}

	// BFS from seed to find closest nodes
	visited := make(map[string]bool)
	queue := []string{seedEntryID}
	order := make([]string, 0, limit)

	for len(queue) > 0 && len(order) < limit {
		curr := queue[0]
		queue = queue[1:]

		if visited[curr] {
			continue
		}
		visited[curr] = true
		order = append(order, curr)

		for _, neighbor := range adjacent[curr] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	// Build pruned graph
	orderSet := make(map[string]bool)
	for _, id := range order {
		orderSet[id] = true
	}

	prunedNodes := make([]types.FlowNode, 0, len(order))
	for _, node := range graph.Nodes {
		if orderSet[node.EntryID] {
			prunedNodes = append(prunedNodes, node)
		}
	}

	prunedEdges := make([]types.FlowEdge, 0)
	for _, edge := range graph.Edges {
		if orderSet[edge.From] && orderSet[edge.To] {
			prunedEdges = append(prunedEdges, edge)
		}
	}

	return &types.FlowGraph{
		Nodes: prunedNodes,
		Edges: prunedEdges,
	}
}
