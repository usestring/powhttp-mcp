// Package config provides configuration loading from environment variables.
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/usestring/powhttp-mcp/pkg/jsoncompact"
)

// Tool output limit defaults
const (
	DefaultSearchLimitValue     = 10
	DefaultQueryLimitValue      = 20
	DefaultClusterLimitValue    = 15
	DefaultExamplesPerItemValue = 3
)

// Processing safety cap defaults
const (
	MaxSearchResultsValue = 10000
	MaxQueryEntriesValue  = 10000
	MaxInferEntriesValue  = 10000
)

// Config holds all configuration for the MCP server.
type Config struct {
	PowHTTPBaseURL       string        // POWHTTP_BASE_URL, default "http://localhost:7777"
	PowHTTPProxyURL      string        // POWHTTP_PROXY_URL, default "http://127.0.0.1:8890"
	HTTPClientTimeout    time.Duration // HTTP_CLIENT_TIMEOUT_MS, default 10000ms (10s)
	RefreshTimeout       time.Duration // REFRESH_TIMEOUT_MS, default 15000ms (15s)
	RefreshInterval      time.Duration // REFRESH_INTERVAL_MS, default 2000ms (2s)
	FreshnessThreshold   time.Duration // FRESHNESS_THRESHOLD_MS, default 500ms
	BootstrapTailLimit   int           // BOOTSTRAP_TAIL_LIMIT, default 20000
	FetchWorkers         int           // FETCH_WORKERS, default 16
	ToolMaxBytesDefault  int           // TOOL_MAX_BYTES_DEFAULT, default 2_000_000
	TLSMaxEventsDefault  int           // TLS_MAX_EVENTS_DEFAULT, default 200
	H2MaxEventsDefault   int           // H2_MAX_EVENTS_DEFAULT, default 200
	EntryCacheMaxItems   int           // ENTRY_CACHE_MAX_ITEMS, default 512
	ResourceMaxBodyBytes int           // RESOURCE_MAX_BODY_BYTES, default 65536 (64KB)
	IndexBody            bool          // INDEX_BODY, default false
	IndexBodyMaxBytes    int           // INDEX_BODY_MAX_BYTES, default 65536

	// Compaction defaults (for AI-optimized responses)
	CompactMaxArrayItems int // COMPACT_MAX_ARRAY_ITEMS
	CompactMaxStringLen  int // COMPACT_MAX_STRING_LEN
	CompactMaxDepth      int // COMPACT_MAX_DEPTH

	// Tool output limits
	DefaultSearchLimit     int // DEFAULT_SEARCH_LIMIT
	DefaultQueryLimit      int // DEFAULT_QUERY_LIMIT
	DefaultClusterLimit    int // DEFAULT_CLUSTER_LIMIT
	DefaultExamplesPerItem int // DEFAULT_EXAMPLES_PER_ITEM

	// Processing safety caps (configurable upper bounds for search/processing space)
	MaxSearchResults int // MAX_SEARCH_RESULTS, default 10000
	MaxQueryEntries  int // MAX_QUERY_ENTRIES, default 10000
	MaxInferEntries  int // MAX_INFER_ENTRIES, default 10000

	// Logging configuration
	LogLevel      string // LOG_LEVEL, default "info"
	LogFile       string // LOG_FILE, default "" (stderr only)
	LogMaxSizeMB  int    // LOG_MAX_SIZE_MB, default 10
	LogMaxBackups int    // LOG_MAX_BACKUPS, default 3
	LogMaxAgeDays int    // LOG_MAX_AGE_DAYS, default 28
	LogCompress   bool   // LOG_COMPRESS, default true
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		PowHTTPBaseURL:       getEnvString("POWHTTP_BASE_URL", "http://localhost:7777"),
		PowHTTPProxyURL:      getEnvString("POWHTTP_PROXY_URL", "http://127.0.0.1:8890"),
		HTTPClientTimeout:    getEnvDurationMs("HTTP_CLIENT_TIMEOUT_MS", 10000),
		RefreshTimeout:       getEnvDurationMs("REFRESH_TIMEOUT_MS", 15000),
		RefreshInterval:      getEnvDurationMs("REFRESH_INTERVAL_MS", 2000),
		FreshnessThreshold:   getEnvDurationMs("FRESHNESS_THRESHOLD_MS", 500),
		BootstrapTailLimit:   getEnvInt("BOOTSTRAP_TAIL_LIMIT", 20000),
		FetchWorkers:         getEnvInt("FETCH_WORKERS", 16),
		ToolMaxBytesDefault:  getEnvInt("TOOL_MAX_BYTES_DEFAULT", 2_000_000),
		TLSMaxEventsDefault:  getEnvInt("TLS_MAX_EVENTS_DEFAULT", 200),
		H2MaxEventsDefault:   getEnvInt("H2_MAX_EVENTS_DEFAULT", 200),
		EntryCacheMaxItems:   getEnvInt("ENTRY_CACHE_MAX_ITEMS", 512),
		ResourceMaxBodyBytes: getEnvInt("RESOURCE_MAX_BODY_BYTES", 65536),
		IndexBody:            getEnvBool("INDEX_BODY", false),
		IndexBodyMaxBytes:    getEnvInt("INDEX_BODY_MAX_BYTES", 65536),

		// Compaction defaults (from jsoncompact package)
		CompactMaxArrayItems: getEnvInt("COMPACT_MAX_ARRAY_ITEMS", jsoncompact.DefaultMaxArrayItems),
		CompactMaxStringLen:  getEnvInt("COMPACT_MAX_STRING_LEN", jsoncompact.DefaultMaxStringLen),
		CompactMaxDepth:      getEnvInt("COMPACT_MAX_DEPTH", jsoncompact.DefaultMaxDepth),

		// Tool output limits
		DefaultSearchLimit:     getEnvInt("DEFAULT_SEARCH_LIMIT", DefaultSearchLimitValue),
		DefaultQueryLimit:      getEnvInt("DEFAULT_QUERY_LIMIT", DefaultQueryLimitValue),
		DefaultClusterLimit:    getEnvInt("DEFAULT_CLUSTER_LIMIT", DefaultClusterLimitValue),
		DefaultExamplesPerItem: getEnvInt("DEFAULT_EXAMPLES_PER_ITEM", DefaultExamplesPerItemValue),

		MaxSearchResults: getEnvInt("MAX_SEARCH_RESULTS", MaxSearchResultsValue),
		MaxQueryEntries:  getEnvInt("MAX_QUERY_ENTRIES", MaxQueryEntriesValue),
		MaxInferEntries:  getEnvInt("MAX_INFER_ENTRIES", MaxInferEntriesValue),

		LogLevel:      getEnvString("LOG_LEVEL", "info"),
		LogFile:       getEnvString("LOG_FILE", ""),
		LogMaxSizeMB:  getEnvInt("LOG_MAX_SIZE_MB", 10),
		LogMaxBackups: getEnvInt("LOG_MAX_BACKUPS", 5),
		LogMaxAgeDays: getEnvInt("LOG_MAX_AGE_DAYS", 28),
		LogCompress:   getEnvBool("LOG_COMPRESS", true),
	}
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		switch v {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return defaultVal
}

func getEnvString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDurationMs(key string, defaultMs int) time.Duration {
	ms := getEnvInt(key, defaultMs)
	return time.Duration(ms) * time.Millisecond
}
