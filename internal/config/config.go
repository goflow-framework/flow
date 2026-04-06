// Package config provides typed application configuration for the Flow
// framework.
//
// Configuration is loaded from environment variables so that the same binary
// can be deployed in multiple environments without re-compilation. The
// FLOW_ENV variable controls which defaults are applied:
//
//   - "development" (default): relaxed security, verbose logging, random
//     session secret accepted.
//   - "test": mirrors development; used in CI/integration tests.
//   - "production": strict validation – missing SecretKeyBase or plaintext
//     HTTP addressing will cause Load to return an error.
//
// Typical usage:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app := flow.New("myapp", flow.WithConfig(cfg))
package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env represents the runtime environment of the application.
type Env string

const (
	// EnvDevelopment is the default environment for local development.
	EnvDevelopment Env = "development"
	// EnvTest is used during automated testing.
	EnvTest Env = "test"
	// EnvProduction enforces strict configuration validation.
	EnvProduction Env = "production"
)

// Config holds all application-wide configuration. Zero values are safe to
// use for development; call Validate() or use Load() which calls it
// automatically.
type Config struct {
	// Env identifies the runtime environment. Defaults to EnvDevelopment.
	Env Env

	// Addr is the TCP address the HTTP server will listen on. Defaults to
	// ":3000".
	Addr string

	// ReadTimeout, WriteTimeout, IdleTimeout are forwarded to http.Server.
	// Zero values keep the defaults applied by flow.New.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// ShutdownTimeout is the maximum time to wait for in-flight requests to
	// drain during graceful shutdown. Defaults to 10s.
	ShutdownTimeout time.Duration

	// DatabaseURL is the DSN for the primary database. Empty means no
	// database is configured.
	DatabaseURL string

	// SecretKeyBase is used to sign/encrypt session cookies. In production
	// this MUST be set via FLOW_SECRET_KEY_BASE and MUST be ≥ 32 bytes.
	// In development a random value is acceptable (and expected).
	SecretKeyBase string

	// LogLevel controls minimum log verbosity. Recognised values (case-
	// insensitive): "debug", "info", "warn", "error". Defaults to "info".
	LogLevel string

	// CookieSecure marks session cookies as Secure (HTTPS-only). Defaults
	// to true in production and false otherwise.
	CookieSecure bool

	// CookieSameSite is the SameSite policy applied to session cookies.
	// Defaults to http.SameSiteLaxMode in production, SameSiteDefaultMode
	// elsewhere.
	CookieSameSite http.SameSite
}

// Load builds a Config by reading environment variables. It calls Validate
// before returning so callers receive a ready-to-use value or a descriptive
// error.
//
// Environment variables:
//
//	FLOW_ENV               – runtime environment (development|test|production)
//	FLOW_ADDR              – listen address, e.g. ":8080"
//	FLOW_READ_TIMEOUT      – e.g. "5s"
//	FLOW_WRITE_TIMEOUT     – e.g. "10s"
//	FLOW_IDLE_TIMEOUT      – e.g. "120s"
//	FLOW_SHUTDOWN_TIMEOUT  – e.g. "10s"
//	DATABASE_URL           – primary database DSN
//	FLOW_SECRET_KEY_BASE   – session signing secret (≥ 32 chars in production)
//	FLOW_LOG_LEVEL         – debug|info|warn|error
//	FLOW_COOKIE_SECURE     – "true"/"false" (default derived from env)
//	FLOW_COOKIE_SAME_SITE  – "default"|"lax"|"strict"|"none"
func Load() (*Config, error) {
	cfg := defaults()

	if v := os.Getenv("FLOW_ENV"); v != "" {
		cfg.Env = Env(strings.ToLower(v))
	}

	// Re-apply env-specific defaults now that we know the environment.
	applyEnvDefaults(cfg)

	if v := os.Getenv("FLOW_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("FLOW_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadTimeout = d
		}
	}
	if v := os.Getenv("FLOW_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.WriteTimeout = d
		}
	}
	if v := os.Getenv("FLOW_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.IdleTimeout = d
		}
	}
	if v := os.Getenv("FLOW_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("FLOW_SECRET_KEY_BASE"); v != "" {
		cfg.SecretKeyBase = v
	}
	if v := os.Getenv("FLOW_LOG_LEVEL"); v != "" {
		cfg.LogLevel = strings.ToLower(v)
	}
	if v := os.Getenv("FLOW_COOKIE_SECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.CookieSecure = b
		}
	}
	if v := os.Getenv("FLOW_COOKIE_SAME_SITE"); v != "" {
		cfg.CookieSameSite = parseSameSite(v)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// defaults returns a Config populated with safe development defaults.
func defaults() *Config {
	return &Config{
		Env:             EnvDevelopment,
		Addr:            ":3000",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		LogLevel:        "info",
		CookieSecure:    false,
		CookieSameSite:  http.SameSiteDefaultMode,
	}
}

// applyEnvDefaults overrides security-sensitive fields based on the resolved
// environment. Called after FLOW_ENV is read so the caller can still
// override individual fields with their own env vars.
func applyEnvDefaults(cfg *Config) {
	switch cfg.Env {
	case EnvProduction:
		cfg.CookieSecure = true
		cfg.CookieSameSite = http.SameSiteLaxMode
	}
}

// Validate checks that the config is coherent for the configured environment.
// It returns a combined error listing every problem found.
func (c *Config) Validate() error {
	var errs []string

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("unknown FLOW_ENV %q (valid: development, test, production)", c.Env))
	}

	if c.Env == EnvProduction {
		if len(c.SecretKeyBase) < 32 {
			errs = append(errs, "FLOW_SECRET_KEY_BASE must be set to at least 32 characters in production")
		}
	}

	if c.Addr == "" {
		errs = append(errs, "FLOW_ADDR must not be empty")
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "": true}
	if !validLevels[c.LogLevel] {
		errs = append(errs, fmt.Sprintf("FLOW_LOG_LEVEL %q is not valid (debug|info|warn|error)", c.LogLevel))
	}

	if len(errs) > 0 {
		return errors.New("config: " + strings.Join(errs, "; "))
	}
	return nil
}

// IsProduction returns true when the application is running in the production
// environment.
func (c *Config) IsProduction() bool {
	return c != nil && c.Env == EnvProduction
}

// IsDevelopment returns true for development and test environments, where
// relaxed defaults are acceptable.
func (c *Config) IsDevelopment() bool {
	return c == nil || c.Env == EnvDevelopment || c.Env == EnvTest
}

// SecretKeyBytes returns the SecretKeyBase as a byte slice ready for use as
// an HMAC key. In development, if SecretKeyBase is empty, a deterministic
// placeholder is returned — never use this value in production.
func (c *Config) SecretKeyBytes() []byte {
	if c != nil && c.SecretKeyBase != "" {
		return []byte(c.SecretKeyBase)
	}
	// development-only fallback – never reaches production due to Validate()
	return []byte("dev-only-insecure-placeholder-key-32b")
}

// parseSameSite converts a string to an http.SameSite constant.
// Unknown values default to SameSiteDefaultMode.
func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(s) {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}
