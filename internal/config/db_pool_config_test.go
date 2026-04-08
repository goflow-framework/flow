package config_test

import (
	"testing"
	"time"

	"github.com/undiegomejia/flow/internal/config"
)

// ---------------------------------------------------------------------------
// DBPoolConfig defaults — development
// ---------------------------------------------------------------------------

func TestDBPool_DevelopmentDefaults(t *testing.T) {
	// Unset any pool env vars so we exercise the package defaults.
	for _, k := range []string{
		"FLOW_ENV",
		"FLOW_DB_MAX_OPEN", "FLOW_DB_MAX_IDLE",
		"FLOW_DB_CONN_MAX_LIFETIME", "FLOW_DB_CONN_MAX_IDLE_TIME",
		"FLOW_SECRET_KEY_BASE",
	} {
		setenv(t, k, "")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBPool.MaxOpenConns != 5 {
		t.Errorf("dev MaxOpenConns = %d, want 5", cfg.DBPool.MaxOpenConns)
	}
	if cfg.DBPool.MaxIdleConns != 5 {
		t.Errorf("dev MaxIdleConns = %d, want 5", cfg.DBPool.MaxIdleConns)
	}
	if cfg.DBPool.ConnMaxLifetime != 0 {
		t.Errorf("dev ConnMaxLifetime = %v, want 0", cfg.DBPool.ConnMaxLifetime)
	}
	if cfg.DBPool.ConnMaxIdleTime != 0 {
		t.Errorf("dev ConnMaxIdleTime = %v, want 0", cfg.DBPool.ConnMaxIdleTime)
	}
}

// ---------------------------------------------------------------------------
// DBPoolConfig defaults — production
// ---------------------------------------------------------------------------

func TestDBPool_ProductionDefaults(t *testing.T) {
	setenv(t,
		"FLOW_ENV", "production",
		"FLOW_SECRET_KEY_BASE", "a-very-long-production-secret-key-value-here",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBPool.MaxOpenConns != 25 {
		t.Errorf("prod MaxOpenConns = %d, want 25", cfg.DBPool.MaxOpenConns)
	}
	if cfg.DBPool.MaxIdleConns != 25 {
		t.Errorf("prod MaxIdleConns = %d, want 25", cfg.DBPool.MaxIdleConns)
	}
	if cfg.DBPool.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("prod ConnMaxLifetime = %v, want 30m", cfg.DBPool.ConnMaxLifetime)
	}
	if cfg.DBPool.ConnMaxIdleTime != 10*time.Minute {
		t.Errorf("prod ConnMaxIdleTime = %v, want 10m", cfg.DBPool.ConnMaxIdleTime)
	}
}

// ---------------------------------------------------------------------------
// Env-var overrides
// ---------------------------------------------------------------------------

func TestDBPool_EnvVarOverrides(t *testing.T) {
	setenv(t,
		"FLOW_DB_MAX_OPEN", "50",
		"FLOW_DB_MAX_IDLE", "10",
		"FLOW_DB_CONN_MAX_LIFETIME", "1h",
		"FLOW_DB_CONN_MAX_IDLE_TIME", "5m",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBPool.MaxOpenConns != 50 {
		t.Errorf("MaxOpenConns = %d, want 50", cfg.DBPool.MaxOpenConns)
	}
	if cfg.DBPool.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.DBPool.MaxIdleConns)
	}
	if cfg.DBPool.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", cfg.DBPool.ConnMaxLifetime)
	}
	if cfg.DBPool.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 5m", cfg.DBPool.ConnMaxIdleTime)
	}
}

// Env-var overrides must win even in production (no silent clamping).
func TestDBPool_EnvVarOverridesProductionDefaults(t *testing.T) {
	setenv(t,
		"FLOW_ENV", "production",
		"FLOW_SECRET_KEY_BASE", "a-very-long-production-secret-key-value-here",
		"FLOW_DB_MAX_OPEN", "100",
		"FLOW_DB_MAX_IDLE", "20",
		"FLOW_DB_CONN_MAX_LIFETIME", "1h30m",
		"FLOW_DB_CONN_MAX_IDLE_TIME", "15m",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBPool.MaxOpenConns != 100 {
		t.Errorf("MaxOpenConns = %d, want 100", cfg.DBPool.MaxOpenConns)
	}
	if cfg.DBPool.MaxIdleConns != 20 {
		t.Errorf("MaxIdleConns = %d, want 20", cfg.DBPool.MaxIdleConns)
	}
	if cfg.DBPool.ConnMaxLifetime != 90*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 1h30m", cfg.DBPool.ConnMaxLifetime)
	}
	if cfg.DBPool.ConnMaxIdleTime != 15*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 15m", cfg.DBPool.ConnMaxIdleTime)
	}
}

// Bad values must be silently ignored (other valid fields still apply).
func TestDBPool_InvalidEnvVarsIgnored(t *testing.T) {
	setenv(t,
		"FLOW_DB_MAX_OPEN", "not-a-number",
		"FLOW_DB_MAX_IDLE", "-5",
		"FLOW_DB_CONN_MAX_LIFETIME", "bad-duration",
		"FLOW_DB_CONN_MAX_IDLE_TIME", "",
	)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should fall back to development defaults (5/5/0/0).
	if cfg.DBPool.MaxOpenConns != 5 {
		t.Errorf("invalid FLOW_DB_MAX_OPEN should leave default 5, got %d", cfg.DBPool.MaxOpenConns)
	}
	if cfg.DBPool.MaxIdleConns != 5 {
		t.Errorf("negative FLOW_DB_MAX_IDLE should be ignored, got %d", cfg.DBPool.MaxIdleConns)
	}
	if cfg.DBPool.ConnMaxLifetime != 0 {
		t.Errorf("invalid FLOW_DB_CONN_MAX_LIFETIME should leave 0, got %v", cfg.DBPool.ConnMaxLifetime)
	}
}
