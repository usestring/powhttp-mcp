// Package logging provides structured logging with file rotation.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds logging configuration.
type Config struct {
	Level      string // Log level: debug, info, warn, error
	FilePath   string // Path to log file (empty = stderr only)
	MaxSizeMB  int    // Max size in MB before rotation
	MaxBackups int    // Max number of old log files to retain
	MaxAgeDays int    // Max age in days to retain old log files
	Compress   bool   // Whether to compress rotated files
}

// DefaultConfig returns sensible defaults for logging.
func DefaultConfig() Config {
	return Config{
		Level:      "info",
		FilePath:   "",
		MaxSizeMB:  100,
		MaxBackups: 3,
		MaxAgeDays: 28,
		Compress:   true,
	}
}

// Setup initializes the global slog logger with the given configuration.
// Returns a cleanup function that should be called on shutdown.
func Setup(cfg Config) (func() error, error) {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var writer io.Writer
	var cleanup func() error

	if cfg.FilePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		lj := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
		writer = lj
		cleanup = lj.Close
	} else {
		writer = os.Stderr
		cleanup = func() error { return nil }
	}

	handler := slog.NewTextHandler(writer, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return cleanup, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
