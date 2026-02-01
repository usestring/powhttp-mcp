package mcpsrv

import (
	"context"
	"net/http"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/config"
)

// serverConfig holds configuration built from options.
type serverConfig struct {
	config     *config.Config
	httpClient *http.Client

	// Logging overrides
	logLevel string
	logFile  string

	// Extension toggles
	disableBuiltinTools   bool
	disableBuiltinPrompts bool

	// Custom extensions - registration callbacks that preserve generic type info
	toolRegistrations     []func(*mcp.Server)
	promptRegistrations   []func(*mcp.Server)
	resourceRegistrations []func(*mcp.Server)

	// Deferred tool registrations that need access to Deps
	deferredToolRegistrations []func(*mcp.Server, *Deps)
}

// Option configures the server.
type Option func(*serverConfig)

// WithLogLevel sets the log level (debug, info, warn, error).
func WithLogLevel(level string) Option {
	return func(cfg *serverConfig) {
		cfg.logLevel = level
	}
}

// WithLogFile sets the log file path.
// If empty, logs are written to stderr only.
func WithLogFile(path string) Option {
	return func(cfg *serverConfig) {
		cfg.logFile = path
	}
}

// WithHTTPClient sets a custom HTTP client for the powhttp API.
// Note: This does not affect the client passed to NewServer.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *serverConfig) {
		cfg.httpClient = c
	}
}

// WithoutBuiltinTools disables all builtin powhttp tools.
// Use this if you want to register only your own tools.
func WithoutBuiltinTools() Option {
	return func(cfg *serverConfig) {
		cfg.disableBuiltinTools = true
	}
}

// WithoutBuiltinPrompts disables all builtin powhttp prompts.
// Use this if you want to register only your own prompts.
func WithoutBuiltinPrompts() Option {
	return func(cfg *serverConfig) {
		cfg.disableBuiltinPrompts = true
	}
}

// WithTool registers a custom tool with the server.
//
// The handler signature must match the MCP SDK pattern:
//
//	func(ctx context.Context, req *mcp.CallToolRequest, input T) (*mcp.CallToolResult, Out, error)
//
// Where T is the input type (will be unmarshaled from JSON) and Out is the
// output type (will be marshaled to JSON).
//
// Example:
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
//	mcpsrv.WithTool(&mcp.Tool{Name: "my_tool", Description: "My tool"}, myHandler)
func WithTool[In, Out any](tool *mcp.Tool, handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)) Option {
	return func(cfg *serverConfig) {
		// Store a callback that calls AddTool with output zero-value check
		cfg.toolRegistrations = append(cfg.toolRegistrations, func(srv *mcp.Server) {
			AddTool(srv, tool, handler)
		})
	}
}

// WithDepsTool registers a custom tool that has access to Deps.
// Use this when your tool needs access to search, cache, or other infrastructure.
//
// The builder receives Deps and returns a handler function.
//
// Example:
//
//	mcpsrv.WithDepsTool(
//	    &mcp.Tool{Name: "search_body", Description: "Search HTTP bodies"},
//	    func(d *mcpsrv.Deps) func(ctx context.Context, req *mcp.CallToolRequest, input MyInput) (*mcp.CallToolResult, MyOutput, error) {
//	        return func(ctx context.Context, req *mcp.CallToolRequest, input MyInput) (*mcp.CallToolResult, MyOutput, error) {
//	            results, _ := d.Search.Search(ctx, &search.SearchRequest{Query: input.Query})
//	            return nil, MyOutput{Count: len(results.Results)}, nil
//	        }
//	    },
//	)
func WithDepsTool[In, Out any](tool *mcp.Tool, builder func(*Deps) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)) Option {
	return func(cfg *serverConfig) {
		cfg.deferredToolRegistrations = append(cfg.deferredToolRegistrations, func(srv *mcp.Server, deps *Deps) {
			handler := builder(deps)
			AddTool(srv, tool, handler)
		})
	}
}

// WithPrompt registers a custom prompt with the server.
//
// The handler signature matches the MCP SDK pattern:
//
//	func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
//
// Example:
//
//	mcpsrv.WithPrompt(
//	    &mcp.Prompt{Name: "my_prompt", Description: "My prompt"},
//	    func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
//	        return &mcp.GetPromptResult{
//	            Description: "My prompt result",
//	            Messages: []*mcp.PromptMessage{
//	                {Role: "user", Content: &mcp.TextContent{Text: "Hello"}},
//	            },
//	        }, nil
//	    },
//	)
func WithPrompt(prompt *mcp.Prompt, handler func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error)) Option {
	return func(cfg *serverConfig) {
		cfg.promptRegistrations = append(cfg.promptRegistrations, func(srv *mcp.Server) {
			srv.AddPrompt(prompt, handler)
		})
	}
}

// WithResourceTemplate registers a custom resource template with the server.
//
// The handler signature matches the MCP SDK pattern:
//
//	func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
//
// Example:
//
//	mcpsrv.WithResourceTemplate(
//	    &mcp.ResourceTemplate{URITemplate: "custom://{id}", Name: "Custom Resource"},
//	    func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
//	        return &mcp.ReadResourceResult{
//	            Contents: []*mcp.ResourceContents{
//	                {URI: req.Params.URI, MIMEType: "application/json", Text: `{"data": "value"}`},
//	            },
//	        }, nil
//	    },
//	)
func WithResourceTemplate(template *mcp.ResourceTemplate, handler func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)) Option {
	return func(cfg *serverConfig) {
		cfg.resourceRegistrations = append(cfg.resourceRegistrations, func(srv *mcp.Server) {
			srv.AddResourceTemplate(template, handler)
		})
	}
}
