<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

powhttp-mcp is an MCP server that provides AI assistants with tools to analyze HTTP traffic captured by [powhttp](https://powhttp.com).

## Commands

```bash
go build ./cmd/powhttp-mcp    # Build
go test ./...                  # Test
go test -v -race -coverprofile=coverage.out ./...  # Test (CI)
```

## Architecture

```
cmd/powhttp-mcp/          Entry point
pkg/
├── client/               powhttp API client
├── mcpsrv/               MCP server wrapper (functional options)
├── types/                Shared response types
├── jsonschema/           JSON schema inference
├── graphql/              GraphQL request body parsing
├── shape/                Unified shape analysis (JSON, XML, CSV, HTML)
├── textquery/            Multi-format query engine (JQ, CSS, XPath, regex, form keys)
└── contenttype/          Content-type detection and classification
internal/
├── mcp/
│   ├── tools/            Tool implementations (17 tools)
│   ├── prompts/          Guided workflows
│   └── server.go         Server initialization
├── indexer/              Full-text search (Roaring bitmaps)
├── query/                JQ expression engine
├── catalog/              API endpoint clustering
├── compare/              TLS fingerprinting (JA3/JA4) and diff
├── flow/                 Request flow tracing
├── schema/               Schema validation (Go, Zod, JSON Schema)
└── cache/                LRU entry cache
```

## Adding a New Tool

1. Create input struct with `jsonschema` tags in `internal/mcp/tools/`:
```go
type MyToolInput struct {
    SessionID string `json:"session_id,omitempty" jsonschema:"Session ID (default: active)"`
    Required  string `json:"required" jsonschema:"required,Description here"`
}
```

2. Implement tool function returning `(*sdkmcp.CallToolResult, ResponseType, error)`:
```go
func ToolMyTool(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input MyToolInput) (*sdkmcp.CallToolResult, MyResponse, error) {
    return func(ctx context.Context, req *sdkmcp.CallToolRequest, input MyToolInput) (*sdkmcp.CallToolResult, MyResponse, error) {
        // Validate input
        if input.Required == "" {
            return nil, MyResponse{}, ErrInvalidInput("required field is missing")
        }

        // Default session
        sessionID := input.SessionID
        if sessionID == "" {
            sessionID = "active"
        }

        // Use dependencies from d.*
        // Return nil for CallToolResult, SDK handles serialization
        return nil, result, nil
    }
}
```

3. Register in `internal/mcp/tools/register.go`:
```go
sdkmcp.AddTool(srv, &sdkmcp.Tool{
    Name:        "powhttp_my_tool",
    Description: "Description for the tool",
}, ToolMyTool(d))
```

## Key Idioms

**Shared types go in `pkg/types/`** for external access by custom tools. Internal implementations stay in `internal/` but expose interfaces via `pkg/types/` when needed.

**Session ID defaults to "active":**
```go
sessionID := input.SessionID
if sessionID == "" {
    sessionID = "active"
}
```

**Bodies are base64-encoded in the API:**
```go
bodyBytes, err := base64.StdEncoding.DecodeString(*entry.Response.Body)
```

**Use coded errors for tool responses:**
```go
return nil, Response{}, ErrInvalidInput("message")
return nil, Response{}, ErrNotFound("resource", id)
return nil, Response{}, WrapPowHTTPError(err)
```

**Check errors with errors.Is/errors.As:**
```go
var apiErr *client.APIError
if errors.As(err, &apiErr) {
    // handle API error
}
```

**Deps struct provides all dependencies:**
```go
d.Client            // powhttp API client
d.Cache             // LRU entry cache
d.Indexer           // full-text search
d.ClusterStore      // stored cluster results
d.Config            // environment config
d.Search            // search engine
d.Fingerprint       // TLS fingerprint engine
d.Diff              // entry diff engine
d.Cluster           // endpoint clustering engine
d.Describe          // endpoint description engine
d.Flow              // request flow tracing engine
d.TextQuery         // multi-format query engine
d.GraphQLParseCache // shared GraphQL parse cache (sync.Map)
```

## Code Style

- No emojis in logs or comments
- Use `errors.Is`/`errors.As` for error checking
- Tests use `stretchr/testify/assert`

## Contributing

PR titles must follow Conventional Commits: `feat:` → minor, `fix:` → patch, `feat!:` → major
