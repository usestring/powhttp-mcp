package mcpsrv

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/mcp/tools"
)

// AddTool registers a tool with the server, validating that the output type's
// zero value passes the SDK's JSON schema check. This catches nil-slice-as-null
// bugs at startup: Go's json.Marshal serializes nil slices as null, but the SDK
// infers "type": "array" from the Go type, so null fails validation at runtime.
//
// If the zero value of Out fails schema validation, AddTool panics with an
// actionable message telling the caller which field to fix.
//
// Use this instead of [sdkmcp.AddTool] to get the additional check.
func AddTool[In, Out any](srv *sdkmcp.Server, t *sdkmcp.Tool, h sdkmcp.ToolHandlerFor[In, Out]) {
	tools.AddTool(srv, t, h)
}
