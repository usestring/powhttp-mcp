package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the default base URL for the powhttp Data API.
const DefaultBaseURL = "http://localhost:7777"

// Client is a powhttp Data API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option is a functional option for configuring the Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(baseURL, "/")
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// New creates a new powhttp API client.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// get performs a GET request and decodes the JSON response.
func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	start := time.Now()

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed",
			slog.String("method", "GET"),
			slog.String("path", path),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		apiErr := c.parseError(resp)
		slog.Debug("HTTP request returned error",
			slog.String("method", "GET"),
			slog.String("path", path),
			slog.Int("status", resp.StatusCode),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		return apiErr
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	slog.Debug("HTTP request completed",
		slog.String("method", "GET"),
		slog.String("path", path),
		slog.Int("status", resp.StatusCode),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	return nil
}

// parseError extracts an APIError from an error response.
func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp errorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
}
