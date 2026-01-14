package prompts

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers all prompts with the MCP server.
func Register(srv *sdkmcp.Server, cfg *Config) {
	// Prompt 1: Compare browser vs program request
	srv.AddPrompt(&sdkmcp.Prompt{
		Name:        "compare_browser_program",
		Description: "RECOMMENDED: Compare browser vs program requests to find anti-bot detection differences. Start here - provides workflow guidance and data shapes without high context cost of fetching resources.",
		Arguments: []*sdkmcp.PromptArgument{
			{
				Name:        "baseline_hint",
				Description: "Search hint for baseline (browser) request",
				Required:    false,
			},
			{
				Name:        "candidate_hint",
				Description: "Search hint for candidate (program) request",
				Required:    false,
			},
		},
	}, HandleCompareBrowserProgram(cfg))

	// Prompt 2: Build API map from captured traffic
	srv.AddPrompt(&sdkmcp.Prompt{
		Name:        "build_api_map",
		Description: "RECOMMENDED: Build an API endpoint catalog from captured traffic. Start here - provides workflow guidance and data shapes without high context cost of fetching resources.",
		Arguments: []*sdkmcp.PromptArgument{
			{
				Name:        "host",
				Description: "Filter by host",
				Required:    false,
			},
			{
				Name:        "process_name",
				Description: "Filter by process name",
				Required:    false,
			},
		},
	}, HandleBuildAPIMap(cfg))

	// Prompt 3: Generate MVP Scraper
	srv.AddPrompt(&sdkmcp.Prompt{
		Name:        "generate_scraper",
		Description: "RECOMMENDED: Generate well-structured MVP scraper from captured traffic. Guides through analysis, architecture, and code generation using tls-client.",
		Arguments: []*sdkmcp.PromptArgument{
			{
				Name:        "usecase",
				Description: "What you want to scrape and why (e.g., 'extract product prices for price monitoring', 'collect news articles for analysis')",
				Required:    false,
			},
			{
				Name:        "domain",
				Description: "Target domain to focus on (e.g., 'example.com', 'api.example.com')",
				Required:    false,
			},
		},
	}, HandleGenerateScraper(cfg))

}
