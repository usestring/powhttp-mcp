// Package mcpsrv provides an extensible MCP server for powhttp.
//
// This package exposes a high-level API for creating and running an MCP server
// with all builtin powhttp tools, prompts, and resources. Users can extend the
// server with custom tools, prompts, and resources using functional options.
//
// # Basic Usage
//
// Create a server with default configuration:
//
//	server, err := mcpsrv.NewServer(client.New())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	server.Run(ctx)
//
// # Extension
//
// Add custom tools using MCP SDK types directly:
//
//	import mcp "github.com/modelcontextprotocol/go-sdk/mcp"
//
//	type MyInput struct {
//	    Query string `json:"query"`
//	}
//
//	type MyOutput struct {
//	    Count int `json:"count"`
//	}
//
//	func myHandler(ctx context.Context, req *mcp.CallToolRequest, input MyInput) (*mcp.CallToolResult, MyOutput, error) {
//	    return nil, MyOutput{Count: 42}, nil
//	}
//
//	server, err := mcpsrv.NewServer(
//	    client.New(),
//	    mcpsrv.WithTool(&mcp.Tool{Name: "my_tool", Description: "My tool"}, myHandler),
//	)
//
// # Configuration
//
// Configure logging and other options:
//
//	server, err := mcpsrv.NewServer(
//	    client.New(),
//	    mcpsrv.WithLogLevel("debug"),
//	    mcpsrv.WithLogFile("/var/log/powhttp-mcp.log"),
//	)
package mcpsrv
