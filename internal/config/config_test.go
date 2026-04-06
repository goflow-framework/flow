package config_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/undiegomejia/flow/internal/config"
)

// setenv sets environment variables for the duration of the test and restores
// the original values via t.Cleanup.
func setenv(t *testing.T, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatal("setenv: pairs must be even")
	}
	for i := 0; i < len(pairs); i += 2 {
		key, val := pairs[i], pairs[i+1]
		old, had := os.LookupEnv(key)
		if err := os.Setenv(key, val); err != nil {
			t.Fatalf("os.Setenv(%q): %v", key, err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	// Ensure any residual env vars from the outer test environment do not
	// interfere.  We only unset the keys we care about.
	for _, k := range []string{
		"FLOW_ENV", "FLOW_ADDR", "FLOW_SECRET_KEY_BASE",
		"FLOW_READ_TIMEOUT", "FLOW_WRITE_TIMEOUT", "FLOW_IDLE_TIMEOUT",
		"FLOW_SHUTDOWN_TIMEOUT", "FLOW_LOG_LEVEL",
		"FLOW_COOKIE_SECURE", "FLOW_COOKIE_SAME_SITE", "DATABASE_URL",
	} {
		old, had := os.LookupEnv(k)
		_ = os.Unsetenv(k)
		k, old, had := k, old, had // capture
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, old)
			}
		})
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env != config.EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, config.EnvDevelopment)
	}
	if cfg.Addr != ":3000" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":3000")
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true in development, want false")
	}
	if cfg.CookieSameSite != http.SameSiteDefaultMode {
		t.Errorf("CookieSameSite = %v, want SameSiteDefaultMode", cfg.CookieSameSite)
	}
}

func TestEnvVarOverrides(t *testing.T) {
	setenv(t,
		"FLOW_ENV", "test",
		"FLOW_ADDR", ":9090",
		"FLOW_READ_TIMEOUT", "3s",
		"FLOW_WRITE_TIMEOUT", "7s",
		"FLOW_IDLE_TIMEOUT", "60s",
		"FLOW_SHUTDOWN_TIMEOUT", "15s",
		"DATABASE_URL", "postgres://localhost/test",
		"FLOW_SECRET_KEY_BASE", "supersecretkeyfortest1234567890ab",
		"FLOW_LOG_LEVEL", "DEBUG",
		"FLOW_COOKIE_SECURE", "true",
		"FLOW_COOKIE_SAME_SITE", "lax",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Env != config.EnvTest {
		t.Errorf("Env = %q, want %q", cfg.Env, config.EnvTest)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want 3s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 7*time.Second {
		t.Errorf("WriteTimeout = %v, want 7s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DatabaseURL = %q, want postgres://...", cfg.DatabaseURL)
	}
	if cfg.SecretKeyBase != "supersecretkeyfortest1234567890ab" {
		t.Errorf("SecretKeyBase not set correctly")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug (lowercased)", cfg.LogLevel)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true")
	}
	if cfg.CookieSameSite != http.SameSiteLaxMode {
		t.Errorf("CookieSameSite = %v, want SameSiteLaxMode", cfg.CookieSameSite)
	}
}

func TestProductionDefaults(t *testing.T) {
	setenv(t,
		"FLOW_ENV", "production",
		"FLOW_SECRET_KEY_BASE", "a-very-long-production-secret-key-value-here",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.CookieSecure {
		t.Error("CookieSecure should default to true in production")
	}
	if cfg.CookieSameSite != http.SameSiteLaxMode {
		t.Errorf("CookieSameSite = %v, want SameSiteLaxMode in production", cfg.CookieSameSite)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	if cfg.IsDevelopment() {
		t.Error("IsDevelopment() = true in production, want false")
	}
}

func TestValidate_ProductionRequiresSecret(t *testing.T) {
	setenv(t,
		"FLOW_ENV", "production",
		// deliberately NOT setting FLOW_SECRET_KEY_BASE
	)
	_ = os.Unsetenv("FLOW_SECRET_KEY_BASE")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing secret in production, got nil")
	}
}

func TestValidate_UnknownEnv(t *testing.T) {
	setenv(t, "FLOW_ENV", "staging")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for unknown FLOW_ENV, got nil")
	}
}

func TestValidate_BadLogLevel(t *testing.T) {
	cfg := &config.Config{
		Env:      config.EnvDevelopment,
		Addr:     ":3000",
		LogLevel: "verbose", // not a valid level
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for bad LogLevel, got nil")
	}
}

func TestSecretKeyBytes_Development(t *testing.T) {
	cfg := &config.Config{Env: config.EnvDevelopment}
	b := cfg.SecretKeyBytes()
	if len(b) == 0 {
		t.Error("SecretKeyBytes should return non-empty slice in development")
	}
}

func TestSecretKeyBytes_WithKey(t *testing.T) {
	cfg := &config.Config{
		Env:           config.EnvDevelopment,
		SecretKeyBase: "my-test-secret-key",
	}
	b := cfg.SecretKeyBytes()
	if string(b) != "my-test-secret-key" {
		t.Errorf("SecretKeyBytes = %q, want %q", string(b), "my-test-secret-key")
	}
}

func TestParseSameSite(t *testing.T) {
	cases := []struct {
		input string
		want  http.SameSite
	}{
		{"lax", http.SameSiteLaxMode},
		{"LAX", http.SameSiteLaxMode},
		{"strict", http.SameSiteStrictMode},
		{"Strict", http.SameSiteStrictMode},
		{"none", http.SameSiteNoneMode},
		{"default", http.SameSiteDefaultMode},
		{"", http.SameSiteDefaultMode},
		{"bogus", http.SameSiteDefaultMode},
	}

	for _, tc := range cases {
		setenv(t,
			"FLOW_ENV", "development",
			"FLOW_COOKIE_SAME_SITE", tc.input,
		)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("input=%q: Load() error = %v", tc.input, err)
		}
		if cfg.CookieSameSite != tc.want {
			t.Errorf("input=%q: CookieSameSite = %v, want %v", tc.input, cfg.CookieSameSite, tc.want)
		}
	}
}

func TestIsDevelopment(t *testing.T) {
	for _, env := range []config.Env{config.EnvDevelopment, config.EnvTest} {
		cfg := &config.Config{Env: env}
		if !cfg.IsDevelopment() {
			t.Errorf("IsDevelopment() = false for env %q", env)
		}
	}
	// nil receiver
	var nilCfg *config.Config
	if !nilCfg.IsDevelopment() {
		t.Error("IsDevelopment() = false for nil config, want true")
	}
}
