package mcp

import (
	"context"
	"log/slog"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// LoggingMiddleware returns middleware that logs all incoming method calls.
func LoggingMiddleware() sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			start := time.Now()

			result, err := next(ctx, method, req)

			duration := time.Since(start)
			attrs := []slog.Attr{
				slog.String("method", method),
				slog.Int64("duration_ms", duration.Milliseconds()),
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				slog.LogAttrs(ctx, slog.LevelError, "method call failed", attrs...)
			} else {
				slog.LogAttrs(ctx, slog.LevelInfo, "method call completed", attrs...)
			}

			return result, err
		}
	}
}
