package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/mcp/tools"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

// Resource URI scheme: powhttp://
// Supported URIs:
//   powhttp://entry/{session}/{entry}
//   powhttp://tls/{connection}
//   powhttp://http2/{conn}/{stream}
//   powhttp://diff/{baseline}/{candidate}
//   powhttp://catalog/{scope_hash}
//   powhttp://flow/{seed}

// registerResources registers resource templates and handlers.
func (s *Server) registerResources() {
	// Register resource templates with their handlers
	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://entry/{session}/{entry}",
		Name:        "HTTP Entry",
		Description: "Full HTTP entry with complete request/response bodies. Use powhttp_get_entry tool first to see schema/summary.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.8,
		},
	}, s.handleResourceEntry)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://tls/{connection}",
		Name:        "TLS Connection",
		Description: "All TLS handshake events for a connection. High context cost - tools already return TLS summaries (version, cipher, JA3/JA4). Only fetch when you need raw handshake frame details.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.3,
		},
	}, s.handleResourceTLS)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://http2/{connection}/{stream}",
		Name:        "HTTP/2 Stream",
		Description: "All HTTP/2 frames for a stream. High context cost - tools already return frame type summaries. Only fetch when you need raw frame-level debugging.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.3,
		},
	}, s.handleResourceHTTP2)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://diff/{baseline}/{candidate}",
		Name:        "Entry Diff",
		Description: "Complete diff between two entries. High context cost - the diff tool already returns structured differences. Only fetch for full raw comparison data.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.4,
		},
	}, s.handleResourceDiff)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://catalog/{scope_hash}",
		Name:        "Endpoint Catalog",
		Description: "Full endpoint catalog with all clusters. High context cost - extract_endpoints tool already returns cluster summaries. Only fetch for complete catalog dump.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.4,
		},
	}, s.handleResourceCatalog)

	s.mcpServer.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "powhttp://flow/{seed}",
		Name:        "Request Flow",
		Description: "Complete request flow graph. High context cost - trace_flow tool already returns the graph structure. Only fetch for raw graph data export.",
		MIMEType:    tools.MimeJSON,
		Annotations: &sdkmcp.Annotations{
			Audience: []sdkmcp.Role{"assistant"},
			Priority: 0.4,
		},
	}, s.handleResourceFlow)
}

// Resource handlers

func (s *Server) handleResourceEntry(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	entry, err := s.deps.Client.GetEntry(ctx, params["session"], params["entry"])
	if err != nil {
		return nil, tools.WrapPowHTTPError(err)
	}

	opts := tools.BodyTransformOptions{
		MaxBytes:   s.deps.Config.ResourceMaxBodyBytes,
		SchemaOnly: false, // Always full body - tool already shows schema
	}

	displayEntry := tools.ToDisplayEntry(entry, opts)
	return toResourceResult(req.Params.URI, displayEntry)
}

func (s *Server) handleResourceTLS(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	connectionID := params["connection"]
	events, err := s.deps.Client.GetTLSConnection(ctx, connectionID)
	if err != nil {
		return nil, tools.WrapPowHTTPError(err)
	}

	content := map[string]any{
		"connection_id": connectionID,
		"events":        events,
	}

	return toResourceResult(req.Params.URI, content)
}

func (s *Server) handleResourceHTTP2(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	connID := params["connection"]
	streamID, _ := strconv.Atoi(params["stream"])

	frames, err := s.deps.Client.GetHTTP2Stream(ctx, connID, streamID)
	if err != nil {
		return nil, tools.WrapPowHTTPError(err)
	}

	content := map[string]any{
		"connection_id": connID,
		"stream_id":     streamID,
		"frames":        frames,
	}

	return toResourceResult(req.Params.URI, content)
}

func (s *Server) handleResourceDiff(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	diffReq := &types.DiffRequest{
		BaselineEntryID:  params["baseline"],
		CandidateEntryID: params["candidate"],
		SessionID:        "active",
	}

	result, err := s.deps.Diff.Diff(ctx, diffReq)
	if err != nil {
		return nil, tools.WrapPowHTTPError(err)
	}

	return toResourceResult(req.Params.URI, result)
}

func (s *Server) handleResourceCatalog(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	scopeHash := params["scope_hash"]
	resp, ok := s.deps.ClusterStore.GetScope(scopeHash)
	if !ok {
		return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
	}

	return toResourceResult(req.Params.URI, resp)
}

func (s *Server) handleResourceFlow(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	params, err := parseResourceURI(req.Params.URI)
	if err != nil {
		return nil, err
	}

	traceReq := &types.TraceRequest{
		SessionID:   "active",
		SeedEntryID: params["seed"],
	}

	graph, err := s.deps.Flow.Trace(ctx, traceReq)
	if err != nil {
		return nil, tools.WrapPowHTTPError(err)
	}

	return toResourceResult(req.Params.URI, graph)
}

// Helper functions

// parseResourceURI extracts parameters from a powhttp:// URI.
func parseResourceURI(uri string) (map[string]string, error) {
	if !strings.HasPrefix(uri, "powhttp://") {
		return nil, tools.ErrInvalidInput("invalid URI scheme: expected powhttp://")
	}

	path := strings.TrimPrefix(uri, "powhttp://")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil, tools.ErrInvalidInput("empty resource path")
	}

	params := make(map[string]string)
	resourceType := parts[0]

	switch resourceType {
	case "entry":
		if len(parts) < 3 {
			return nil, tools.ErrInvalidInput("entry URI requires session and entry ID")
		}
		params["session"] = parts[1]
		params["entry"] = parts[2]
		if len(parts) >= 4 && parts[3] == "schema" {
			params["schema"] = "true"
		}

	case "tls":
		if len(parts) < 2 {
			return nil, tools.ErrInvalidInput("tls URI requires connection ID")
		}
		params["connection"] = parts[1]

	case "http2":
		if len(parts) < 3 {
			return nil, tools.ErrInvalidInput("http2 URI requires connection ID and stream ID")
		}
		params["connection"] = parts[1]
		params["stream"] = parts[2]

	case "diff":
		if len(parts) < 3 {
			return nil, tools.ErrInvalidInput("diff URI requires baseline and candidate IDs")
		}
		params["baseline"] = parts[1]
		params["candidate"] = parts[2]

	case "catalog":
		if len(parts) < 2 {
			return nil, tools.ErrInvalidInput("catalog URI requires scope hash")
		}
		params["scope_hash"] = parts[1]

	case "flow":
		if len(parts) < 2 {
			return nil, tools.ErrInvalidInput("flow URI requires seed entry ID")
		}
		params["seed"] = parts[1]

	default:
		return nil, tools.ErrInvalidInput(fmt.Sprintf("unknown resource type: %s", resourceType))
	}

	return params, nil
}

// toResourceResult serializes content to a ReadResourceResult.
func toResourceResult(uri string, content any) (*sdkmcp.ReadResourceResult, error) {
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing resource: %w", err)
	}

	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: tools.MimeJSON,
				Text:     string(data),
			},
		},
	}, nil
}
