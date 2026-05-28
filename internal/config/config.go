// Package config provides configuration loading, parsing, and validation for
// turnfly. Configuration can come from environment variables, CLI flags, or
// an optional config file.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for turnfly.
type Config struct {
	// TURN server configuration.
	TURNPort         int    // UDP/TCP port for TURN (default: 3478)
	TURNRealm        string // TURN realm (required)
	TURNSharedSecret string // HMAC shared secret for ephemeral credentials (required)

	// HTTP control API configuration.
	HTTPPort      int    // HTTP API port (default: 8080)
	AdminAPIToken string // Admin API bearer token (required)

	// Metrics configuration.
	MetricsAddr string // Prometheus metrics listen address (default: :9090)

	// Fly.io configuration.
	FlyAppName string // Fly app name
	FlyOrg     string // Fly organization

	// Relay mode configuration (experimental).
	RelayMode  bool     // Enable experimental relay-pair mode
	RelayPeers []string // List of relay peer addresses

	// Log level.
	LogLevel string // debug, info, warn, error (default: info)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		TURNPort:    3478,
		HTTPPort:    8080,
		MetricsAddr: ":9090",
		LogLevel:    "info",
	}
}

// LoadFromEnv populates config from environment variables.
// Environment variables take precedence over defaults but not CLI flags.
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("TURN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.TURNPort = p
		}
	}
	if v := os.Getenv("TURN_REALM"); v != "" {
		c.TURNRealm = v
	}
	if v := os.Getenv("TURN_SHARED_SECRET"); v != "" {
		c.TURNSharedSecret = v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.HTTPPort = p
		}
	}
	if v := os.Getenv("ADMIN_API_TOKEN"); v != "" {
		c.AdminAPIToken = v
	}
	if v := os.Getenv("METRICS_ADDR"); v != "" {
		c.MetricsAddr = v
	}
	if v := os.Getenv("FLY_APP_NAME"); v != "" {
		c.FlyAppName = v
	}
	if v := os.Getenv("FLY_ORG"); v != "" {
		c.FlyOrg = v
	}
	if v := os.Getenv("RELAY_MODE"); v != "" {
		c.RelayMode = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("RELAY_PEERS"); v != "" {
		c.RelayPeers = strings.Split(v, ",")
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
}

// Validate checks that the configuration is valid and returns an error if
// required fields are missing or values are out of range.
func (c *Config) Validate() error {
	var errs []error

	if c.TURNRealm == "" {
		errs = append(errs, errors.New("TURN_REALM is required"))
	}
	if c.TURNSharedSecret == "" {
		errs = append(errs, errors.New("TURN_SHARED_SECRET is required"))
	}
	if c.AdminAPIToken == "" {
		errs = append(errs, errors.New("ADMIN_API_TOKEN is required"))
	}

	if c.TURNPort < 1 || c.TURNPort > 65535 {
		errs = append(errs, fmt.Errorf("TURN_PORT must be between 1 and 65535, got %d", c.TURNPort))
	}
	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		errs = append(errs, fmt.Errorf("HTTP_PORT must be between 1 and 65535, got %d", c.HTTPPort))
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.LogLevel] {
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", c.LogLevel))
	}

	if c.TURNPort == c.HTTPPort {
		errs = append(errs, fmt.Errorf("TURN_PORT and HTTP_PORT must not be the same (both set to %d)", c.TURNPort))
	}

	return errors.Join(errs...)
}
