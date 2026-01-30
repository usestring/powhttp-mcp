package mcpsrv

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/catalog"
	"github.com/usestring/powhttp-mcp/internal/compare"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/internal/flow"
	"github.com/usestring/powhttp-mcp/internal/indexer"
	"github.com/usestring/powhttp-mcp/internal/logging"
	"github.com/usestring/powhttp-mcp/internal/mcp"
	"github.com/usestring/powhttp-mcp/internal/mcp/tools"
	"github.com/usestring/powhttp-mcp/internal/search"
	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/textquery"
)

// Server is the powhttp MCP server.
// It wraps the internal implementation and provides extension points.
type Server struct {
	internal   *mcp.Server
	indexer    *indexer.Indexer
	deps       *Deps
	logCleanup func() error
}

// NewServer creates a new MCP server with builtin powhttp tools.
//
// The client parameter is required and provides access to the powhttp API.
// Use functional options to configure logging, add custom tools, etc.
func NewServer(c *client.Client, opts ...Option) (*Server, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}

	// Build configuration from options
	cfg := &serverConfig{
		config: config.Load(), // Load defaults from environment
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Setup logging
	logCfg := logging.Config{
		Level:      cfg.config.LogLevel,
		FilePath:   cfg.config.LogFile,
		MaxSizeMB:  cfg.config.LogMaxSizeMB,
		MaxBackups: cfg.config.LogMaxBackups,
		MaxAgeDays: cfg.config.LogMaxAgeDays,
		Compress:   cfg.config.LogCompress,
	}
	if cfg.logLevel != "" {
		logCfg.Level = cfg.logLevel
	}
	if cfg.logFile != "" {
		logCfg.FilePath = cfg.logFile
	}
	logCleanup, err := logging.Setup(logCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logging: %w", err)
	}

	// Create infrastructure
	entryCache, err := cache.NewEntryCache(cfg.config.EntryCacheMaxItems)
	if err != nil {
		return nil, fmt.Errorf("failed to create entry cache: %w", err)
	}

	idx := indexer.New(c, entryCache, cfg.config)
	clusterStore := catalog.NewClusterStore()

	// Create engines
	searchEngine := search.New(idx, entryCache)
	fpEngine := compare.NewFingerprintEngine(c, entryCache, cfg.config)
	diffEngine := compare.NewDiffEngine(fpEngine)
	clusterEngine := catalog.NewClusterEngine(idx, cfg.config, clusterStore)
	describeEngine := catalog.NewDescribeEngine(idx, c, entryCache, cfg.config, clusterStore)
	flowEngine := flow.NewFlowEngine(idx, cfg.config)
	textQueryEngine := textquery.NewEngine()

	// Create deps for internal tools and custom tools
	toolDeps := &tools.Deps{
		Client:       c,
		Indexer:      idx,
		Cache:        entryCache,
		Config:       cfg.config,
		Search:       searchEngine,
		Fingerprint:  fpEngine,
		Diff:         diffEngine,
		Cluster:      clusterEngine,
		Describe:     describeEngine,
		ClusterStore: clusterStore,
		Flow:         flowEngine,
		TextQuery:    textQueryEngine,
	}

	// Create public deps (same values, different type for public API)
	deps := &Deps{
		Client:       c,
		Indexer:      idx,
		Cache:        entryCache,
		Config:       cfg.config,
		Search:       searchEngine,
		Fingerprint:  fpEngine,
		Diff:         diffEngine,
		Cluster:      clusterEngine,
		Describe:     describeEngine,
		ClusterStore: clusterStore,
		Flow:         flowEngine,
		TextQuery:    textQueryEngine,
	}

	// Build internal server options
	var internalOpts []mcp.ServerOption
	if !cfg.disableBuiltinTools {
		internalOpts = append(internalOpts, mcp.WithBuiltinTools())
	}
	if !cfg.disableBuiltinPrompts {
		internalOpts = append(internalOpts, mcp.WithBuiltinPrompts())
	}

	// Add custom extension registration callbacks
	for _, fn := range cfg.toolRegistrations {
		internalOpts = append(internalOpts, mcp.WithCustomRegistration(fn))
	}
	for _, fn := range cfg.promptRegistrations {
		internalOpts = append(internalOpts, mcp.WithCustomRegistration(fn))
	}
	for _, fn := range cfg.resourceRegistrations {
		internalOpts = append(internalOpts, mcp.WithCustomRegistration(fn))
	}

	// Add deferred tool registrations (tools that need Deps access)
	for _, fn := range cfg.deferredToolRegistrations {
		fn := fn // capture for closure
		internalOpts = append(internalOpts, mcp.WithCustomRegistration(func(srv *sdkmcp.Server) {
			fn(srv, deps)
		}))
	}

	// Create internal server
	internal, err := mcp.NewServer(toolDeps, internalOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return &Server{
		internal:   internal,
		indexer:    idx,
		deps:       deps,
		logCleanup: logCleanup,
	}, nil
}

// Run starts the MCP server with stdio transport.
// It also starts background refresh for all sessions.
// The server runs until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	go s.indexer.StartBackgroundRefresh(ctx)
	return s.internal.Run(ctx)
}

// Close cleans up server resources.
func (s *Server) Close() error {
	if s.logCleanup != nil {
		return s.logCleanup()
	}
	return nil
}

// Deps returns the dependencies for building custom tools.
func (s *Server) Deps() *Deps {
	return s.deps
}
