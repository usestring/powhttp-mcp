# MCP Package

The `mcp` package implements a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [powhttp](https://powhttp.com), providing AI assistants with tools to analyze HTTP traffic, compare requests, and generate scrapers.

## Overview

This package wraps the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) and exposes powhttp functionality through:

- **17 Tools** - Structured functions for HTTP traffic analysis
- **6 Resource Templates** - Access to raw data (entries, TLS, HTTP/2, diffs, etc.)
- **4 Prompts** - Guided workflows for common tasks

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
| `powhttp_query_body` | Extract specific fields from bodies using JQ expressions |
| `powhttp_infer_schema` | Infer merged schema from multiple entry bodies with field statistics |
| `powhttp_graphql_operations` | Cluster GraphQL traffic by operation name and type |
| `powhttp_graphql_inspect` | Parse and inspect individual GraphQL operations |
| `powhttp_graphql_errors` | Extract and categorize GraphQL errors from responses |

See tool source files in `tools/` for detailed input/output schemas.

### Token Optimization

Tools are optimized to minimize context usage by default:

**`powhttp_search_entries`**
- Returns thin results by default (entry_id, URL, method, status, http_version, content-type hint)
- Set `include_details: true` only when filtering by TLS/HTTP2/process info
- `sizes.resp_content_type` helps identify JSON responses without fetching bodies

**`powhttp_get_entry`**
- `include_headers: false` (default) - omits headers to save tokens
- `body_mode`: `compact` (default - arrays trimmed to 3 items), `schema` (JSON schema only), `full` (complete body)

**`powhttp_query_body`**
- Extract specific fields directly using JQ expressions
- No need to fetch full bodies for data extraction
- Supports `deduplicate: true` to remove duplicate values

**`powhttp_infer_schema`**
- Infers a merged schema from multiple entry bodies with field statistics (frequency, required/optional, formats, enums)
- Handles all content types: JSON/YAML get JSON Schema, others get structural outlines
- Use before `powhttp_query_body` to discover available fields and their types

### GraphQL Tools

For GraphQL APIs, use the dedicated tools instead of `powhttp_extract_endpoints` (which collapses all GraphQL operations into one cluster):

**`powhttp_graphql_operations`**
- Clusters by operation name and type (query/mutation/subscription)
- Returns counts, error counts, fields, and example entry IDs
- The GraphQL equivalent of `powhttp_extract_endpoints`

**`powhttp_graphql_inspect`**
- Parses variables_schema, response_schema, and field statistics for an operation
- Accepts `entry_ids` or `operation_name`

**`powhttp_graphql_errors`**
- Groups errors by message with paths and extensions
- Distinguishes partial failures (data + errors) from full failures (null data + errors)
- Hints point to the most-errored operation for actionable debugging

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
| `base_prompt` | START HERE: Essential guide for efficient tool usage and token optimization |
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
