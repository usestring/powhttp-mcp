package prompts

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleGenerateScraper implements the scraper generation workflow.
func HandleGenerateScraper(cfg *Config) func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		args := req.Params.Arguments

		useCase := "General data extraction"
		domain := ""
		if args != nil {
			if v, ok := args["usecase"]; ok && v != "" {
				useCase = v
			}
			if v, ok := args["domain"]; ok {
				domain = v
			}
		}

		var sb strings.Builder

		// ---------------------------------------------------------
		// 1. BASE PROMPT: MODERN GOLANG STANDARDS
		// ---------------------------------------------------------
		sb.WriteString("# Base Prompt: Modern Golang Standards\n\n")
		sb.WriteString("**Role**: Act as a Principal Golang Engineer and Architect.\n")
		sb.WriteString("**Objective**: Write production-grade, idiomatic Go code based on the specific scraping requirements below.\n\n")

		sb.WriteString("### 📐 Code Quality Standards (Strict Enforcement)\n")
		sb.WriteString("1. **Version**: Target **Go 1.24+**. Use modern features (`min`/`max`, `slices` package).\n")
		sb.WriteString("2. **Idiomatic Style**:\n")
		sb.WriteString("   - Follow \"Effective Go\". Use `camelCase` for internal, `PascalCase` for exported.\n")
		sb.WriteString("   - Prefer guard clauses (early returns) over nested indentation.\n")
		sb.WriteString("3. **Error Handling**:\n")
		sb.WriteString("   - **Never** ignore errors.\n")
		sb.WriteString("   - Use `fmt.Errorf(\"failed to ...: %w\", err)` to wrap errors with context.\n")
		sb.WriteString("   - Use `errors.Is()` / `errors.As()` for checks.\n")
		sb.WriteString("4. **Concurrency & Context**:\n")
		sb.WriteString("   - All blocking functions (requests, I/O) MUST accept `context.Context` as the first argument.\n")
		sb.WriteString("   - Handle cancellation and timeouts gracefully.\n")
		sb.WriteString("   - Use `errgroup` or `sourcegraph/conc` for synchronization.\n")
		sb.WriteString("5. **Dependency Management**:\n")
		sb.WriteString("   - Prefer StdLib (`encoding/json`, `log/slog`) for logic.\n")
		sb.WriteString("   - **EXCEPTION**: For HTTP Transport, you MUST use `bogdanfinn/tls-client` (fhttp) to mimic browsers.\n")
		sb.WriteString("   - **STRICTLY BANNED**: `io/ioutil` (deprecated).\n\n")

		// ---------------------------------------------------------
		// 2. CONTEXT & MISSION
		// ---------------------------------------------------------
		sb.WriteString("# Mission: Stealth Scraper Implementation\n\n")
		sb.WriteString("### 🎯 Objective\n")
		sb.WriteString(fmt.Sprintf("- **Goal**: %s\n", useCase))
		if domain != "" {
			sb.WriteString(fmt.Sprintf("- **Target Domain**: %s\n", domain))
		}
		sb.WriteString(fmt.Sprintf("- **Debug Proxy**: `%s` (Required for verification)\n\n", cfg.PowHTTPProxyURL))

		// ---------------------------------------------------------
		// 3. MCP ANALYSIS PHASE
		// ---------------------------------------------------------
		sb.WriteString("### 🕵️ Phase 1: Reconnaissance (MCP Tools)\n")
		sb.WriteString("Before coding, map the territory using `powhttp` tools:\n")
		sb.WriteString("1. **Find Endpoints**:\n")
		if domain != "" {
			sb.WriteString(fmt.Sprintf("   `powhttp_extract_endpoints(scope={\"host\": \"%s\"})`\n", domain))
		} else {
			sb.WriteString("   `powhttp_extract_endpoints()`\n")
		}
		sb.WriteString("1b. **Discover Auth Patterns** (use all three approaches):\n")
		sb.WriteString("   - `powhttp_describe_endpoint(cluster_id=\"...\")` - Check `auth_signals` for cookies, bearer tokens, custom auth headers\n")
		sb.WriteString("   - `powhttp_search_entries(filters={header_contains: \"authorization\"})` - Find Bearer/Basic auth across all traffic\n")
		sb.WriteString("   - `powhttp_search_entries(filters={header_contains: \"x-api-key\"})` - Find API key auth\n")
		sb.WriteString("   - `powhttp_trace_flow(seed_entry_id=\"...\")` - Check `edge_type_summary` for `auth_chain`, `session_cookie_origin`, `same_auth` edges to map auth flow\n")
		sb.WriteString("2. **Analyze Structure**:\n")
		sb.WriteString("   Pick the relevant `cluster_id` and run: `powhttp_describe_endpoint(cluster_id=\"...\")`\n")
		sb.WriteString("3. **Get Fingerprint Data**:\n")
		sb.WriteString("   Pick a successful `entry_id` and run: `powhttp_fingerprint(entry_id=\"...\")`\n")
		sb.WriteString("   *Note: Capture the exact Header Order and Pseudo-Header Order.*\n")
		sb.WriteString("4. **Inspect Raw Request**:\n")
		sb.WriteString("   Run: `powhttp_get_entry(entry_id=\"...\")` to get exact headers, cookies, and body shape.\n")
		sb.WriteString("   For full request/response bodies, fetch the resource URI returned in the response.\n")
		sb.WriteString("5. **Initial Type Check**:\n")
		sb.WriteString("   Analyze the response body JSON to determine field types (int vs float vs string).\n\n")

		// ---------------------------------------------------------
		// 4. IMPLEMENTATION PHASE
		// ---------------------------------------------------------
		sb.WriteString("### 🏗️ Phase 2: Implementation\n\n")

		sb.WriteString("#### A. Project Initialization\n")
		sb.WriteString("Run these commands to set up the project structure:\n")
		sb.WriteString("```bash\n")
		sb.WriteString("mkdir -p cmd/scraper internal/client internal/scraper internal/config internal/models internal/storage\n")
		sb.WriteString("go mod init scraper\n")
		sb.WriteString("go get [github.com/bogdanfinn/tls-client@latest](https://github.com/bogdanfinn/tls-client@latest)\n")
		sb.WriteString("go get [github.com/bogdanfinn/fhttp@latest](https://github.com/bogdanfinn/fhttp@latest)\n")
		sb.WriteString("go get golang.org/x/time/rate\n")
		sb.WriteString("go get [github.com/sourcegraph/conc](https://github.com/sourcegraph/conc)\n")
		sb.WriteString("go get [github.com/dgraph-io/badger/v4](https://github.com/dgraph-io/badger/v4)\n")
		sb.WriteString("```\n\n")

		// ---------------------------------------------------------
		// CLIENT FACTORY
		// ---------------------------------------------------------
		sb.WriteString("#### B. `internal/client/client.go` (TLS & Proxy)\n")
		sb.WriteString("Use this **EXACT** pattern. This configuration is critical for passing `powhttp_diff_entries` checks.\n\n")
		sb.WriteString("```go\n")
		sb.WriteString("package client\n\n")
		sb.WriteString("import (\n")
		sb.WriteString("    \"fmt\"\n")
		sb.WriteString("    http \"[github.com/bogdanfinn/fhttp](https://github.com/bogdanfinn/fhttp)\"\n")
		sb.WriteString("    \"[github.com/bogdanfinn/fhttp/cookiejar](https://github.com/bogdanfinn/fhttp/cookiejar)\"\n")
		sb.WriteString("    tls_client \"[github.com/bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client)\"\n")
		sb.WriteString("    \"[github.com/bogdanfinn/tls-client/profiles](https://github.com/bogdanfinn/tls-client/profiles)\"\n")
		sb.WriteString(")\n\n")
		sb.WriteString("func New(proxyURL string) (tls_client.HttpClient, error) {\n")
		sb.WriteString("    jar, _ := cookiejar.New(nil)\n")
		sb.WriteString("    opts := []tls_client.HttpClientOption{\n")
		sb.WriteString("        tls_client.WithTimeoutSeconds(30),\n")
		sb.WriteString("        tls_client.WithClientProfile(profiles.Chrome_133_PSK), // CRITICAL: Matches Chrome 133+\n")
		sb.WriteString("        tls_client.WithNotFollowRedirects(),\n")
		sb.WriteString("        tls_client.WithCookieJar(jar),\n")
		sb.WriteString("        tls_client.WithRandomTLSExtensionOrder(), // Helps evade fingerprinting\n")
		sb.WriteString("        tls_client.WithDisableHttp3(),            // Chrome typically doesn't use H3 yet - helps match fingerprints\n")
		sb.WriteString("    }\n\n")
		sb.WriteString("    if proxyURL != \"\" {\n")
		sb.WriteString("        opts = append(opts, tls_client.WithProxyUrl(proxyURL))\n")
		sb.WriteString("    }\n\n")
		sb.WriteString("    // Use NewNoopLogger to keep stdout clean unless debugging\n")
		sb.WriteString("    return tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)\n")
		sb.WriteString("}\n")
		sb.WriteString("```\n\n")

		sb.WriteString("#### C. Header Logic (Crucial)\n")
		sb.WriteString("You MUST use `http.HeaderOrderKey` and `http.PHeaderOrderKey` from `fhttp`.\n")
		sb.WriteString("1. Copy the headers **exactly** as they appear in `powhttp_get_entry`.\n")
		sb.WriteString("2. Populate `HeaderOrderKey` and `PHeaderOrderKey` with the exact order found in the captured traffic.\n\n")

		sb.WriteString("⚠️ **IMPORTANT: Do NOT manually set these headers**:\n")
		sb.WriteString("- `Cookie` / `Set-Cookie` - Handled automatically by `tls_client.WithCookieJar(jar)`\n")
		sb.WriteString("- `Content-Length` - Calculated automatically by the HTTP client\n")
		sb.WriteString("- `Host` - Derived from the request URL automatically\n")
		sb.WriteString("- `Connection` - Managed by the HTTP/2 transport layer\n")
		sb.WriteString("- Same as the stdlib net/http\n")
		sb.WriteString("- Do not set HTTP2 pseudo-headers manually, only set the ordering.\n")
		sb.WriteString("The cookie jar persists cookies across requests within the same client session. ")
		sb.WriteString("If you need to pre-seed cookies, use `jar.SetCookies(url, cookies)` on the cookie jar directly.\n\n")

		sb.WriteString("#### D. `cmd/scraper/main.go` Wiring\n")
		sb.WriteString("Wire everything up using `slog` and `context`; following best practices for structured logging and error handling.\n")
		sb.WriteString("```go\n")
		sb.WriteString("func main() {\n")
		sb.WriteString("    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))\n")
		sb.WriteString("    ctx := context.Background()\n\n")
		sb.WriteString(fmt.Sprintf("    // Use the PowHTTP Proxy for verification: %s\n", cfg.PowHTTPProxyURL))
		sb.WriteString(fmt.Sprintf("    c, err := client.New(\"%s\")\n", cfg.PowHTTPProxyURL))
		sb.WriteString("    if err != nil { panic(err) }\n\n")
		sb.WriteString("    // ... Instantiate scraper and run ...\n")
		sb.WriteString("}\n")
		sb.WriteString("```\n\n")

		// ---------------------------------------------------------
		// 5. PAGINATION PATTERNS
		// ---------------------------------------------------------
		sb.WriteString("### 🔄 Phase 3: Pagination Patterns\n")
		sb.WriteString("Select the correct pattern based on the API type:\n\n")
		sb.WriteString("1. **Sequential (Cursor-based)**: Use for `next_page` logic. Wrap in `for cursor != \"\"`.\n")
		sb.WriteString("2. **Concurrent (Known Pages)**: Use `sourcegraph/conc/pool`. NEVER use manual `sync.WaitGroup`.\n")
		sb.WriteString("3. **Streaming (Channel)**: Use for processing results (DB save) in real-time.\n\n")

		// ---------------------------------------------------------
		// 6. VALIDATION & TYPES (EMPHASIZED)
		// ---------------------------------------------------------
		sb.WriteString("### 🧪 Phase 4: Type Verification & Validation\n")
		sb.WriteString("You must strictly validate that your Go structs match the real traffic to avoid `json: cannot unmarshal` errors.\n\n")

		sb.WriteString("#### Step 1: Run the Scraper\n")
		sb.WriteString("Execute your generated code once to trigger a request through the proxy.\n\n")

		sb.WriteString("#### Step 2: Validate Schema (The 'Unit Test')\n")
		sb.WriteString("Use the `powhttp_validate_schema` tool to test your Struct against the *actual* captured response body.\n")
		sb.WriteString("1. Copy your Go response struct definition.\n")
		sb.WriteString("2. Run this tool:\n")
		sb.WriteString("   `powhttp_validate_schema(entry_ids=[\"<YOUR_SCRAPER_ENTRY_ID>\"], schema=\"type Response struct { ... }\", schema_format=\"go_struct\", target=\"response\")`\n")
		sb.WriteString("3. **Analyze Result**:\n")
		sb.WriteString("   - If `valid: true`: Your types are correct.\n")
		sb.WriteString("   - If `valid: false`: Read the `errors` field. It will say things like `\"expected string, got number at field 'id'\"`. **Fix your struct immediately.**\n\n")

		sb.WriteString("#### Step 3: Validate Stealth\n")
		sb.WriteString("Ensure you aren't leaking identity:\n")
		sb.WriteString("`powhttp_diff_entries(baseline_entry_id=\"<REAL_BROWSER_ID>\", candidate_entry_id=\"<YOUR_SCRAPER_ENTRY_ID>\")`\n")
		sb.WriteString("- If Diff shows `Header Order Mismatch` -> Fix `HeaderOrderKey`.\n")
		sb.WriteString("- If Diff shows `TLS Fingerprint Mismatch` -> Check `profiles.Chrome_133_PSK`.\n\n")

		// ---------------------------------------------------------
		// 7. DATA STORAGE
		// ---------------------------------------------------------
		sb.WriteString("### 💾 Phase 5: Structured Data Storage\n")
		sb.WriteString("All scraped data MUST be persisted in a structured format. Choose the appropriate storage based on complexity:\n\n")

		sb.WriteString("#### Option A: JSON Lines (Simple)\n")
		sb.WriteString("For simple use cases with small datasets, use JSON Lines format (`.jsonl`).\n")
		sb.WriteString("Create a thread-safe writer in `internal/storage/jsonl.go` with `Write(v any)` and `Close()` methods.\n\n")

		sb.WriteString("#### Option B: BadgerDB (Complex)\n")
		sb.WriteString("For complex use cases requiring indexing, querying, deduplication, or large datasets, use `github.com/dgraph-io/badger/v4`.\n")
		sb.WriteString("Create a store in `internal/storage/store.go` with `Put(key, v)`, `Get(key, v)`, `Exists(key)`, and `Close()` methods.\n")
		sb.WriteString("Use composite keys (e.g., `product:<id>`, `review:<productID>:<reviewID>`) for efficient lookups.\n")
		sb.WriteString("Store scrape progress under `meta:` prefix for resumability.\n\n")

		sb.WriteString("#### When to Use Each Option\n")
		sb.WriteString("| Use Case | Storage |\n")
		sb.WriteString("|----------|----------|\n")
		sb.WriteString("| Simple one-off scrape, small dataset | JSON Lines |\n")
		sb.WriteString("| Need deduplication by ID | BadgerDB |\n")
		sb.WriteString("| Incremental/resumable scraping | BadgerDB |\n")
		sb.WriteString("| Large datasets (>100k records) | BadgerDB |\n")
		sb.WriteString("| Need to query/filter locally | BadgerDB |\n")
		sb.WriteString("| Export to external system | JSON Lines |\n")

		return &sdkmcp.GetPromptResult{
			Description: "Generates a production-ready Go scraper with strict type validation instructions",
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: sb.String()},
				},
			},
		}, nil
	}
}
