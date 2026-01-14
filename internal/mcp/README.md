# MCP Package

The `mcp` package implements a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [powhttp](https://powhttp.com), providing AI assistants with tools to analyze HTTP traffic, compare requests, and generate scrapers.

## Overview

This package wraps the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) and exposes powhttp functionality through:

- **12 Tools** - Structured functions for HTTP traffic analysis
- **6 Resource Templates** - Access to raw data (entries, TLS, HTTP/2, diffs, etc.)
- **3 Prompts** - Guided workflows for common tasks

## Architecture

```
mcp/
├── server.go      - Server struct, functional options, Run()
├── resources.go   - Resource templates + URI parsing
├── middleware.go  - Request logging middleware
└── tools/         - Tool implementations
    ├── register.go    - Tool registration
    ├── helpers.go     - Shared utilities
    ├── errors.go      - Coded errors
    └── *.go           - Individual tool handlers
└── prompts/       - Prompt implementations
    ├── register.go    - Prompt registration
    └── *.go           - Individual prompt handlers
```

## Quick Start

```go
server, err := mcp.NewServer(
    mcp.WithClient(client),
    mcp.WithIndexer(indexer),
    mcp.WithCache(cache),
    mcp.WithConfig(config),
    mcp.WithSearch(searchEngine),
    mcp.WithFingerprint(fingerprintEngine),
    mcp.WithDiff(diffEngine),
    mcp.WithCluster(clusterEngine),
    mcp.WithDescribe(describeEngine),
    mcp.WithClusterStore(clusterStore),
    mcp.WithFlow(flowEngine),
)

err = server.Run(ctx) // stdio transport
```

## Tools

| Tool | Description |
|------|-------------|
| `powhttp_sessions_list` | List all sessions with entry counts |
| `powhttp_session_active` | Get the currently active session |
| `powhttp_search_entries` | Search entries with filters and free text |
| `powhttp_get_entry` | Get full details of a specific entry |
| `powhttp_get_tls` | Get TLS handshake events for a connection |
| `powhttp_get_http2_stream` | Get HTTP/2 frame details for a stream |
| `powhttp_fingerprint` | Generate HTTP, TLS, and HTTP/2 fingerprints |
| `powhttp_diff_entries` | Compare two entries to find detection differences |
| `powhttp_extract_endpoints` | Cluster entries into endpoint groups |
| `powhttp_describe_endpoint` | Generate detailed endpoint description |
| `powhttp_trace_flow` | Trace related requests around a seed entry |
| `powhttp_validate_schema` | Validate entry bodies against a schema |

See tool source files in `tools/` for detailed input/output schemas.

## Resources

Resources provide access to raw data. Use sparingly as they have high context cost.

| URI Template | Description |
|--------------|-------------|
| `powhttp://entry/{session}/{entry}` | Full entry with complete bodies |
| `powhttp://tls/{connection}` | All TLS handshake events |
| `powhttp://http2/{connection}/{stream}` | All HTTP/2 frames |
| `powhttp://diff/{baseline}/{candidate}` | Complete diff between entries |
| `powhttp://catalog/{scope_hash}` | Full cluster catalog |
| `powhttp://flow/{seed}` | Complete flow graph |

**Context Cost Guidance:**
- **Tools** return summaries - low context cost, use these first
- **Resources** return full data - high context cost, use selectively

## Prompts

| Prompt | Description |
|--------|-------------|
| `compare_browser_program` | Compare browser vs program requests to find anti-bot detection differences |
| `build_api_map` | Build an API endpoint catalog from captured traffic |
| `generate_scraper` | Generate POC Go scraper from captured traffic |

## Error Handling

The package uses coded errors in `tools/errors.go`:

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Resource not found (404 from API) |
| `INVALID_INPUT` | Invalid tool input |
| `POWHTTP_ERROR` | General PowHTTP API error |
| `TIMEOUT` | Request timeout |

## Detection Priority Guide

When comparing browser vs program requests:

| Priority | Vector | Impact |
|----------|--------|--------|
| CRITICAL | TLS Fingerprint (JA3/JA4) | Blocks 80%+ of bots |
| HIGH | HTTP/2 pseudo-header order | Common in modern sites |
| HIGH | HTTP/2 SETTINGS frame order | Often overlooked |
| MEDIUM | Header presence/order | Detectable pattern |
| LOW | Header value differences | Usually less critical |
