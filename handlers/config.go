package handlers

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime knobs of the proxy. Every field has a safe default
// so the pool works with only RPC_LIST set.
type Config struct {
	// MaxAttempts is the number of upstream tries per request before the pool
	// gives up (RPC_MAX_ATTEMPTS). Attempts go to distinct upstreams first.
	MaxAttempts int
	// RetryBackoff is the base delay before retrying an upstream that was
	// already tried for this request (RPC_RETRY_BACKOFF_MS).
	RetryBackoff time.Duration
	// UpstreamTimeout bounds one upstream call (UPSTREAM_TIMEOUT_SECONDS).
	UpstreamTimeout time.Duration
	// TotalTimeout bounds one client request, retries included
	// (REQUEST_TIMEOUT_SECONDS). Keep it below the HTTP server WriteTimeout.
	TotalTimeout time.Duration
	// LogBodies adds the JSON-RPC request and response bodies to the debug
	// logs (LOG_BODIES). Only has effect with LOG_LEVEL=debug.
	LogBodies bool
	// LogBodyMaxBytes truncates logged bodies; 0 means no limit
	// (LOG_BODY_MAX_BYTES).
	LogBodyMaxBytes int
}

// DefaultConfig returns the defaults documented in the README.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:     3,
		RetryBackoff:    100 * time.Millisecond,
		UpstreamTimeout: 10 * time.Second,
		TotalTimeout:    25 * time.Second,
		LogBodies:       false,
		LogBodyMaxBytes: 64 * 1024,
	}
}

var cfg = DefaultConfig()

// SetConfig replaces the active configuration and applies the upstream timeout.
func SetConfig(c Config) {
	if c.MaxAttempts < 1 {
		c.MaxAttempts = 1
	}
	cfg = c
	httpClient.Timeout = c.UpstreamTimeout
}

// GetConfig returns the active configuration.
func GetConfig() Config {
	return cfg
}

// LoadConfigFromEnv reads the optional tuning variables. Invalid values keep
// the default and are reported in the returned warnings.
func LoadConfigFromEnv() (Config, []string) {
	c := DefaultConfig()
	var warnings []string

	intVar := func(name string, min int, set func(int)) {
		raw := strings.TrimSpace(os.Getenv(name))
		if raw == "" {
			return
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < min {
			warnings = append(warnings, fmt.Sprintf("%s=%q is invalid (integer >= %d expected), using the default", name, raw, min))
			return
		}
		set(v)
	}

	intVar("RPC_MAX_ATTEMPTS", 1, func(v int) { c.MaxAttempts = v })
	intVar("RPC_RETRY_BACKOFF_MS", 0, func(v int) { c.RetryBackoff = time.Duration(v) * time.Millisecond })
	intVar("UPSTREAM_TIMEOUT_SECONDS", 1, func(v int) { c.UpstreamTimeout = time.Duration(v) * time.Second })
	intVar("REQUEST_TIMEOUT_SECONDS", 1, func(v int) { c.TotalTimeout = time.Duration(v) * time.Second })
	intVar("LOG_BODY_MAX_BYTES", 0, func(v int) { c.LogBodyMaxBytes = v })

	if raw := strings.TrimSpace(os.Getenv("LOG_BODIES")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("LOG_BODIES=%q is invalid (true or false expected), using the default", raw))
		} else {
			c.LogBodies = v
		}
	}

	return c, warnings
}

// ParseLogLevel maps LOG_LEVEL to a slog level. Unknown values fall back to
// info and report ok=false.
func ParseLogLevel(raw string) (level slog.Level, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, true
	case "debug", "trace":
		return slog.LevelDebug, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}
