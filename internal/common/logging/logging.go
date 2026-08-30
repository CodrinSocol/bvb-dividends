// Package logging builds the application's structured logger.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/config"
)

// Config contains logging-specific configuration.
type Config struct {
	Level  string `env:"LOG_LEVEL"  envDefault:"info"  validate:"omitempty,oneof=debug info warn error"`
	Format string `env:"LOG_FORMAT" envDefault:"json"  validate:"omitempty,oneof=json text"`
}

func (cfg Config) slogLevel() slog.Level {
	switch strings.ToLower(cfg.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// replaceAttr expands an error attribute into its message and its stack.
//
// Errors carry a stack because the application wraps them with
// github.com/cockroachdb/errors; without this the JSON handler would render
// only the message, and the stack that makes an unexpected failure diagnosable
// would be dropped on the floor.
func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindAny {
		return a
	}

	err, ok := a.Value.Any().(error)
	if !ok {
		return a
	}

	return slog.Group(a.Key,
		slog.String("message", err.Error()),
		slog.String("trace", fmt.Sprintf("%+v", err)),
	)
}

func newHandler(cfg Config) slog.Handler {
	options := &slog.HandlerOptions{
		AddSource:   true,
		Level:       cfg.slogLevel(),
		ReplaceAttr: replaceAttr,
	}

	if strings.EqualFold(cfg.Format, "text") {
		return slog.NewTextHandler(os.Stderr, options)
	}

	return slog.NewJSONHandler(os.Stdout, options)
}

// NewLogger creates the application's logger and installs it as the default,
// so that a package which was handed no logger still writes somewhere useful.
func NewLogger() (*slog.Logger, error) {
	cfg, err := config.Load[Config]()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	log := slog.New(newHandler(cfg))
	slog.SetDefault(log)

	return log, nil
}
