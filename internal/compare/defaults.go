// Package compare provides fingerprint generation and diff capabilities for HTTP entries.
package compare

// DefaultIgnoreHeaders are headers that commonly vary and are usually noise.
// These are typically set by servers or infrastructure and don't reflect
// meaningful differences between browser and program requests.
var DefaultIgnoreHeaders = []string{
	"date",
	"x-request-id",
	"x-correlation-id",
	"x-trace-id",
	"x-amzn-requestid",
	"x-amzn-trace-id",
	"cf-ray",
	"x-cache",
	"age",
	"expires",
	"last-modified",
	"etag",
}

// DefaultIgnoreQueryKeys are query parameters that commonly vary.
// These are typically cache-busters or timestamps that don't reflect
// meaningful differences between requests.
var DefaultIgnoreQueryKeys = []string{
	"_",
	"t",
	"ts",
	"timestamp",
	"time",
	"rand",
	"random",
	"nonce",
	"cb",
	"cachebuster",
}
