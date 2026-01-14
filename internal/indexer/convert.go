package indexer

import (
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

// FromSessionEntry creates EntryMeta from a powhttp SessionEntry.
func FromSessionEntry(entry *client.SessionEntry) *EntryMeta {
	meta := &EntryMeta{
		EntryID:     entry.ID,
		TsMs:        entry.Timings.StartedAt,
		URL:         entry.URL,
		Host:        extractHost(entry.URL),
		Path:        extractPath(entry.URL),
		HTTPVersion: entry.HTTPVersion,
	}

	// Request fields
	if entry.Request.Method != nil {
		meta.Method = *entry.Request.Method
	}

	// Response fields
	if entry.Response != nil && entry.Response.StatusCode != nil {
		meta.Status = *entry.Response.StatusCode
	}

	// Process info
	if entry.Process != nil {
		meta.PID = entry.Process.PID
		if entry.Process.Name != nil {
			meta.ProcessName = *entry.Process.Name
		}
	}

	// TLS info
	if entry.TLS.ConnectionID != nil {
		meta.TLSConnectionID = *entry.TLS.ConnectionID
	}
	if entry.TLS.JA3 != nil {
		meta.JA3 = entry.TLS.JA3.Hash
	}
	if entry.TLS.JA4 != nil {
		meta.JA4 = entry.TLS.JA4.Hashed
	}

	// HTTP/2 info
	if entry.HTTP2 != nil {
		meta.H2ConnectionID = entry.HTTP2.ConnectionID
		meta.H2StreamID = entry.HTTP2.StreamID
	}

	// Header names (lowercase)
	meta.HeaderNamesLower = extractHeaderNames(entry.Request.Headers)
	if entry.Response != nil {
		meta.HeaderNamesLower = append(meta.HeaderNamesLower, extractHeaderNames(entry.Response.Headers)...)
	}

	// Header values (all name:value pairs for indexing)
	meta.HeaderValues = extractHeaderValues(entry.Request.Headers)
	if entry.Response != nil {
		meta.HeaderValues = append(meta.HeaderValues, extractHeaderValues(entry.Response.Headers)...)
	}

	// Auth fields for flow tracing
	meta.AuthHeader = entry.Request.Headers.Get("authorization")
	meta.Cookies = extractSessionCookies(entry.Request.Headers.Get("cookie"))
	meta.APIKeys = extractAPIKeys(entry.Request.Headers)
	if entry.Response != nil {
		meta.SetCookies = extractSetCookies(entry.Response.Headers)
	}

	// Body sizes
	meta.ReqBodyBytes = computeBodySize(entry.Request.Body)
	if entry.Response != nil {
		meta.RespBodyBytes = computeBodySize(entry.Response.Body)
	}

	return meta
}

// extractHost parses host from URL.
func extractHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

// extractPath parses path from URL.
func extractPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Path
}

// extractHeaderNames gets lowercase header names.
func extractHeaderNames(headers client.Headers) []string {
	seen := make(map[string]struct{}, len(headers))
	result := make([]string, 0, len(headers))

	for _, pair := range headers {
		if len(pair) >= 1 {
			name := strings.ToLower(pair[0])
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				result = append(result, name)
			}
		}
	}

	return result
}

// extractHeaderValues gets all header name:value pairs with lowercase names.
func extractHeaderValues(headers client.Headers) []HeaderValue {
	result := make([]HeaderValue, 0, len(headers))

	for _, pair := range headers {
		if len(pair) >= 2 {
			result = append(result, HeaderValue{
				Name:  strings.ToLower(pair[0]),
				Value: pair[1],
			})
		}
	}

	return result
}

// sessionCookiePatterns contains patterns for session-related cookie names.
var sessionCookiePatterns = []string{
	"session", "sid", "auth", "token", "jwt",
}

// commonSessionCookies contains exact names of common framework session cookies.
var commonSessionCookies = map[string]bool{
	"jsessionid":         true,
	"phpsessid":          true,
	"asp.net_sessionid":  true,
	"connect.sid":        true,
	"_session":           true,
	"_session_id":        true,
}

// isSessionCookie checks if a cookie name is session-related.
func isSessionCookie(name string) bool {
	nameLower := strings.ToLower(name)

	// Check exact matches for common framework cookies
	if commonSessionCookies[nameLower] {
		return true
	}

	// Check patterns
	for _, pattern := range sessionCookiePatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	return false
}

// extractSessionCookies parses Cookie header and returns session-related cookies.
func extractSessionCookies(cookieHeader string) map[string]string {
	if cookieHeader == "" {
		return nil
	}

	result := make(map[string]string)

	// Parse "name=value; name2=value2" format
	pairs := strings.Split(cookieHeader, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		idx := strings.Index(pair, "=")
		if idx <= 0 {
			continue
		}

		name := strings.TrimSpace(pair[:idx])
		value := strings.TrimSpace(pair[idx+1:])

		if isSessionCookie(name) {
			result[name] = value
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// apiKeyHeaders contains header names that typically hold API keys.
var apiKeyHeaders = map[string]bool{
	"x-api-key":      true,
	"x-auth-token":   true,
	"x-access-token": true,
}

// extractAPIKeys extracts API key headers from request headers.
func extractAPIKeys(headers client.Headers) map[string]string {
	result := make(map[string]string)

	for _, pair := range headers {
		if len(pair) >= 2 {
			nameLower := strings.ToLower(pair[0])
			if apiKeyHeaders[nameLower] {
				result[nameLower] = pair[1]
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// extractSetCookies extracts session-related Set-Cookie headers from response.
func extractSetCookies(headers client.Headers) map[string]string {
	result := make(map[string]string)

	for _, pair := range headers {
		if len(pair) >= 2 && strings.ToLower(pair[0]) == "set-cookie" {
			// Parse "name=value; attributes..." format
			cookieValue := pair[1]
			idx := strings.Index(cookieValue, "=")
			if idx <= 0 {
				continue
			}

			name := strings.TrimSpace(cookieValue[:idx])

			// Find end of value (before attributes)
			valueEnd := strings.Index(cookieValue[idx+1:], ";")
			var value string
			if valueEnd == -1 {
				value = strings.TrimSpace(cookieValue[idx+1:])
			} else {
				value = strings.TrimSpace(cookieValue[idx+1 : idx+1+valueEnd])
			}

			if isSessionCookie(name) {
				result[name] = value
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// computeBodySize calculates body size from base64 encoded string.
func computeBodySize(encoded *string) int {
	if encoded == nil || *encoded == "" {
		return 0
	}

	// Calculate decoded size from base64 length
	// Base64 encoding increases size by ~4/3, so decoded = encoded * 3/4
	// Account for padding
	n := len(*encoded)
	padding := 0
	if n > 0 && (*encoded)[n-1] == '=' {
		padding++
		if n > 1 && (*encoded)[n-2] == '=' {
			padding++
		}
	}

	decoded := (n * 3 / 4) - padding

	// Validate by attempting decode (handles malformed input)
	actual, err := base64.StdEncoding.DecodeString(*encoded)
	if err != nil {
		return decoded // Best estimate if decode fails
	}

	return len(actual)
}
