// Package logging builds the service's structured logger.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a logger writing to standard error.
//
// The text format is for reading during development; the JSON format is the
// default, so a deployed service emits something a log pipeline can index.
func New(level, format string) *slog.Logger {
	options := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(os.Stderr, options)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, options)
	}
	return slog.New(handler)
}

// parseLevel maps a level name to a slog level, defaulting to info for an
// unrecognised name rather than failing to start over a log setting.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
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
