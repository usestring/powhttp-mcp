// Command example-body-search demonstrates a custom MCP tool that searches
// HTTP request/response bodies using the builtin search index.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usestring/powhttp-mcp/pkg/client"
	"github.com/usestring/powhttp-mcp/pkg/mcpsrv"
	"github.com/usestring/powhttp-mcp/pkg/types"
)

type BodySearchInput struct {
	Query  string `json:"query" jsonschema:"required,Text to search for in request/response bodies"`
	Host   string `json:"host,omitempty" jsonschema:"Filter by host"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max results (default: 10)"`
	InBody string `json:"in_body,omitempty" jsonschema:"Search in: request, response, or both (default: both)"`
}

type BodySearchOutput struct {
	Matches []BodyMatch `json:"matches"`
	Total   int         `json:"total"`
}

type BodyMatch struct {
	EntryID string `json:"entry_id"`
	URL     string `json:"url"`
	Method  string `json:"method"`
	Status  int    `json:"status"`
	FoundIn string `json:"found_in"`
	Snippet string `json:"snippet"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server, err := mcpsrv.NewServer(
		client.New(),
		mcpsrv.WithDepsTool(
			&mcp.Tool{
				Name:        "search_body",
				Description: "Search for text in HTTP request/response bodies",
			},
			bodySearchHandler,
		),
	)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	slog.Info("starting MCP server with body search tool")
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func bodySearchHandler(d *mcpsrv.Deps) func(context.Context, *mcp.CallToolRequest, BodySearchInput) (*mcp.CallToolResult, BodySearchOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input BodySearchInput) (*mcp.CallToolResult, BodySearchOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 10
		}

		// Use search engine directly
		searchReq := &types.SearchRequest{
			SessionID: "active",
			Limit:     100,
		}
		if input.Host != "" {
			searchReq.Filters = &types.SearchFilters{Host: input.Host}
		}

		resp, err := d.Search.Search(ctx, searchReq)
		if err != nil {
			return nil, BodySearchOutput{}, err
		}

		var matches []BodyMatch
		queryLower := strings.ToLower(input.Query)

		for _, result := range resp.Results {
			if len(matches) >= limit {
				break
			}

			// Get entry from cache or fetch
			entry, ok := d.Cache.Get(result.Summary.EntryID)
			if !ok {
				entry, err = d.Client.GetEntry(ctx, "active", result.Summary.EntryID)
				if err != nil {
					continue
				}
			}

			// Search request body
			if input.InBody == "" || input.InBody == "both" || input.InBody == "request" {
				if entry.Request.Body != nil {
					if match := searchBody(entry.Request.Body, queryLower, result.Summary, "request"); match != nil {
						matches = append(matches, *match)
						continue
					}
				}
			}

			// Search response body
			if input.InBody == "" || input.InBody == "both" || input.InBody == "response" {
				if entry.Response != nil && entry.Response.Body != nil {
					if match := searchBody(entry.Response.Body, queryLower, result.Summary, "response"); match != nil {
						matches = append(matches, *match)
					}
				}
			}
		}

		return nil, BodySearchOutput{Matches: matches, Total: len(matches)}, nil
	}
}

func searchBody(encodedBody *string, query string, summary *types.EntrySummary, foundIn string) *BodyMatch {
	body, err := client.DecodeBody(encodedBody)
	if err != nil {
		return nil
	}

	bodyStr := string(body)
	idx := strings.Index(strings.ToLower(bodyStr), query)
	if idx < 0 {
		return nil
	}

	return &BodyMatch{
		EntryID: summary.EntryID,
		URL:     summary.URL,
		Method:  summary.Method,
		Status:  summary.Status,
		FoundIn: foundIn,
		Snippet: extractSnippet(bodyStr, idx, len(query)),
	}
}

func extractSnippet(text string, matchIdx, matchLen int) string {
	const contextSize = 50

	start := matchIdx - contextSize
	if start < 0 {
		start = 0
	}

	end := matchIdx + matchLen + contextSize
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\r", " ")
	snippet = strings.ReplaceAll(snippet, "\t", " ")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}

	return snippet
}
