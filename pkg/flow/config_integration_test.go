package flow_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/undiegomejia/flow/internal/config"
	flowpkg "github.com/undiegomejia/flow/pkg/flow"

	_ "modernc.org/sqlite" // register sqlite driver for WithConfig DB auto-open tests
)

func TestWithConfig_AppliesTransportFields(t *testing.T) {
	cfg := &config.Config{
		Env:             config.EnvDevelopment,
		Addr:            ":8888",
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    6 * time.Second,
		IdleTimeout:     90 * time.Second,
		ShutdownTimeout: 20 * time.Second,
		LogLevel:        "info",
	}

	app := flowpkg.New("test", flowpkg.WithConfig(cfg))

	if app.Addr != ":8888" {
		t.Errorf("Addr = %q, want :8888", app.Addr)
	}
	if app.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want 3s", app.ReadTimeout)
	}
	if app.WriteTimeout != 6*time.Second {
		t.Errorf("WriteTimeout = %v, want 6s", app.WriteTimeout)
	}
	if app.IdleTimeout != 90*time.Second {
		t.Errorf("IdleTimeout = %v, want 90s", app.IdleTimeout)
	}
	if app.ShutdownTimeout != 20*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 20s", app.ShutdownTimeout)
	}
}

func TestWithConfig_ProductionCookieFlags(t *testing.T) {
	cfg := &config.Config{
		Env:             config.EnvProduction,
		Addr:            ":443",
		SecretKeyBase:   "a-very-long-production-secret-key-value-here",
		LogLevel:        "info",
		CookieSecure:    true,
		CookieSameSite:  http.SameSiteLaxMode,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	app := flowpkg.New("prod", flowpkg.WithConfig(cfg))

	if !app.Sessions.CookieSecure {
		t.Error("Sessions.CookieSecure = false in production config, want true")
	}
	if app.Sessions.CookieSameSite != http.SameSiteLaxMode {
		t.Errorf("Sessions.CookieSameSite = %v, want SameSiteLaxMode", app.Sessions.CookieSameSite)
	}
}

func TestWithConfig_NilIsNoop(t *testing.T) {
	app := flowpkg.New("test", flowpkg.WithConfig(nil))

	// Defaults from New() should be untouched.
	if app.Addr != ":3000" {
		t.Errorf("Addr = %q after nil config, want :3000", app.Addr)
	}
}

func TestWithConfig_OverriddenBySubsequentOption(t *testing.T) {
	cfg := &config.Config{
		Env:             config.EnvDevelopment,
		Addr:            ":8888",
		LogLevel:        "info",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	app := flowpkg.New("test",
		flowpkg.WithConfig(cfg),
		flowpkg.WithAddr(":9999"), // should win
	)

	if app.Addr != ":9999" {
		t.Errorf("Addr = %q, want :9999 (option after WithConfig should win)", app.Addr)
	}
}

// TestWithConfig_DatabaseURL_AutoOpensSQLite verifies that WithConfig
// automatically opens a BunAdapter and attaches it to the App when
// DatabaseURL is set to a sqlite DSN.
func TestWithConfig_DatabaseURL_AutoOpensSQLite(t *testing.T) {
	cfg := &config.Config{
		Env:             config.EnvDevelopment,
		Addr:            ":3000",
		LogLevel:        "info",
		DatabaseURL:     "sqlite://file::memory:?cache=shared",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	app := flowpkg.New("db-test", flowpkg.WithConfig(cfg))

	if app.Bun() == nil {
		t.Fatal("Bun() = nil after WithConfig with DatabaseURL set, want non-nil *bun.DB")
	}
}

// TestWithConfig_EmptyDatabaseURL_NilBun verifies that when DatabaseURL is
// empty no BunAdapter is attached and Bun() returns nil.
func TestWithConfig_EmptyDatabaseURL_NilBun(t *testing.T) {
	cfg := &config.Config{
		Env:             config.EnvDevelopment,
		Addr:            ":3000",
		LogLevel:        "info",
		DatabaseURL:     "", // no DB configured
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	app := flowpkg.New("no-db-test", flowpkg.WithConfig(cfg))

	if app.Bun() != nil {
		t.Fatal("Bun() should be nil when DatabaseURL is empty")
	}
}
