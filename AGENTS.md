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

# Agents Guide for powhttp-mcp

## Project Overview

**powhttp-mcp** is a Model Context Protocol (MCP) server that provides AI assistants with tools to analyze HTTP traffic captured by [powhttp](https://powhttp.com). It enables LLMs to:

- Search and inspect HTTP requests/responses
- Compare browser vs program traffic to identify anti-bot detection
- Generate TLS and HTTP/2 fingerprints
- Cluster API endpoints from captured traffic
- Trace request flows (redirects, dependent calls)
- Validate response schemas
- Generate POC Go scrapers

## Architecture

- **Language**: Go 1.24.5
- **Protocol**: Model Context Protocol (MCP) via stdio
- **API Client**: Connects to powhttp API (default: `http://localhost:7777`)
- **Tools**: 12 MCP tools for HTTP traffic analysis
- **Resources**: 6 resource templates for raw data access
- **Prompts**: 3 guided workflows

## Code Style Guidelines

### General

- Follow standard Go conventions and idioms
- Use meaningful variable names
- Keep functions focused and small
- Write clear comments for complex logic

### Logging

- No emojis in log output or code comments
- Keep comments technical and informative

### Error Handling

- Always check errors
- Provide context when wrapping errors
- Use `errors.Is`/`errors.As` for error checking
- Use coded errors for MCP tool responses (see `internal/mcp/tools/errors.go`)

## Key Directories

```
powhttp-mcp/
├── cmd/powhttp-mcp/     - Main entry point
├── internal/
│   ├── mcp/             - MCP server implementation
│   │   ├── tools/       - 12 MCP tools
│   │   └── prompts/     - 3 guided prompts
│   ├── catalog/         - Endpoint clustering
│   ├── compare/         - Fingerprinting and diff
│   ├── flow/            - Request flow tracing
│   ├── indexer/         - Full-text search
│   └── schema/          - Schema validation
└── pkg/
    ├── client/          - powhttp API client
    └── mcpsrv/          - MCP server wrapper
```

## Environment Variables

- `POWHTTP_BASE_URL` - powhttp API base URL (default: `http://localhost:7777`)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `LOG_FILE` - Path to log file (default: stderr only)

## Testing

Run tests with:
```bash
go test ./...
```

Build with:
```bash
go build ./cmd/powhttp-mcp
```
