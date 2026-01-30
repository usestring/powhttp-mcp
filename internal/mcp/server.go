package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/mcp/prompts"
	"github.com/usestring/powhttp-mcp/internal/mcp/tools"
)

// Server wraps the MCP server with powhttp-specific components.
type Server struct {
	mcpServer *sdkmcp.Server
	deps      *tools.Deps

	// Extension toggles
	enableBuiltinTools   bool
	enableBuiltinPrompts bool

	// Custom extension registration callbacks
	customRegistrations []func(*sdkmcp.Server)
}

// ServerOption is a functional option for configuring the Server.
type ServerOption func(*Server)

// WithBuiltinTools enables the builtin powhttp tools.
func WithBuiltinTools() ServerOption {
	return func(s *Server) {
		s.enableBuiltinTools = true
	}
}

// WithBuiltinPrompts enables the builtin powhttp prompts.
func WithBuiltinPrompts() ServerOption {
	return func(s *Server) {
		s.enableBuiltinPrompts = true
	}
}

// WithCustomRegistration adds a custom registration callback.
// The callback receives the underlying MCP server and can register
// tools, prompts, or resources directly.
func WithCustomRegistration(fn func(*sdkmcp.Server)) ServerOption {
	return func(s *Server) {
		s.customRegistrations = append(s.customRegistrations, fn)
	}
}

// NewServer creates a new MCP server with the provided dependencies and options.
func NewServer(deps *tools.Deps, opts ...ServerOption) (*Server, error) {
	if deps == nil {
		return nil, fmt.Errorf("deps is required")
	}

	s := &Server{deps: deps}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create MCP server
	s.mcpServer = sdkmcp.NewServer(
		&sdkmcp.Implementation{
			Name:    "powhttp-mcp",
			Version: "1.0.0",
		},
		nil,
	)

	// Register logging middleware
	s.mcpServer.AddReceivingMiddleware(LoggingMiddleware())

	// Create prompt config
	promptCfg := &prompts.Config{
		PowHTTPProxyURL:  deps.Config.PowHTTPProxyURL,
		BodyIndexEnabled: deps.Config.IndexBody,
	}

	// Register builtin capabilities if enabled
	if s.enableBuiltinTools {
		tools.Register(s.mcpServer, deps)
		s.registerResources()
	}
	if s.enableBuiltinPrompts {
		prompts.Register(s.mcpServer, promptCfg)
	}

	// Execute custom registration callbacks
	for _, fn := range s.customRegistrations {
		fn(s.mcpServer)
	}

	return s, nil
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &sdkmcp.StdioTransport{})
}

// MCPServer returns the underlying MCP server for testing.
func (s *Server) MCPServer() *sdkmcp.Server {
	return s.mcpServer
}
