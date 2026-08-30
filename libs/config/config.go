// Package config loads the service's settings from the environment.
//
// Everything deployment-specific arrives through the environment, so no
// credential is ever committed. The Java service this replaces kept its
// database password in application.properties, in the repository.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full set of settings both binaries read.
type Config struct {
	DatabaseURL string
	AutoMigrate bool
	API         API
	BVB         BVB
	Log         Log
}

// API configures the HTTP server.
type API struct {
	Addr                string
	AllowedOrigins      []string
	ReadHeaderTimeout   time.Duration
	RequestTimeout      time.Duration
	ShutdownGracePeriod time.Duration
}

// BVB configures the upstream SOAP client.
type BVB struct {
	Endpoint string
	Timeout  time.Duration
	Attempts int
}

// Log configures the structured logger.
type Log struct {
	Level  string
	Format string
}

// Load reads the configuration, applying defaults suited to local development.
//
// It fails only on a value that is present but unusable; a missing optional
// value takes its default. DATABASE_URL is the one setting with no sensible
// default, since guessing at a database is worse than saying it is missing.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		API: API{
			Addr:                envString("API_ADDR", ":8080"),
			AllowedOrigins:      envList("API_CORS_ALLOWED_ORIGINS", []string{"http://localhost:4200"}),
			ReadHeaderTimeout:   10 * time.Second,
			RequestTimeout:      30 * time.Second,
			ShutdownGracePeriod: 15 * time.Second,
		},
		BVB: BVB{
			Endpoint: envString("BVB_ENDPOINT", "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx"),
			Attempts: 3,
		},
		Log: Log{
			Level:  envString("LOG_LEVEL", "info"),
			Format: envString("LOG_FORMAT", "json"),
		},
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}

	var err error
	if cfg.AutoMigrate, err = envBool("DB_AUTO_MIGRATE", true); err != nil {
		return Config{}, err
	}
	if cfg.BVB.Timeout, err = envDuration("BVB_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.API.RequestTimeout, err = envDuration("API_REQUEST_TIMEOUT", cfg.API.RequestTimeout); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// envString returns the variable's value, or fallback when it is unset.
func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// envList splits a comma-separated variable, or returns fallback when unset.
func envList(name string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

// envBool parses a boolean variable, or returns fallback when unset.
func envBool(name string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", name, raw)
	}
	return value, nil
}

// envDuration parses a duration variable, or returns fallback when unset.
func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 30s, got %q", name, raw)
	}
	return value, nil
}
