package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/mcpsrv"
)

func main() {
	// Set up context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create powhttp API client
	// Base URL and HTTP client timeout are configured via environment variables
	// (POWHTTP_BASE_URL defaults to http://localhost:7777)
	powClient := client.New()

	// Create MCP server with all builtin tools
	// Configuration is loaded from environment variables:
	// - LOG_LEVEL: debug, info, warn, error (default: info)
	// - LOG_FILE: path to log file (default: stderr only)
	// - POWHTTP_BASE_URL: powhttp API base URL
	// - etc. (see internal/config for all options)
	server, err := mcpsrv.NewServer(powClient)
	if err != nil {
		slog.Error("failed to create MCP server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	// Run the server with stdio transport
	slog.Info("starting powhttp MCP server on stdio")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
