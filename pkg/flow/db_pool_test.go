package flow_test

import (
	"testing"
	"time"

	"github.com/undiegomejia/flow/internal/config"
	"github.com/undiegomejia/flow/internal/orm"
	flowpkg "github.com/undiegomejia/flow/pkg/flow"

	_ "modernc.org/sqlite" // register sqlite driver
)

// ---------------------------------------------------------------------------
// WithConfig wires pool settings through to the sql.DB
// ---------------------------------------------------------------------------

func TestWithConfig_DBPool_Applied(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Env:         config.EnvDevelopment,
		Addr:        ":3000",
		LogLevel:    "info",
		DatabaseURL: "sqlite://file::memory:?cache=shared",
		DBPool: config.DBPoolConfig{
			MaxOpenConns:    12,
			MaxIdleConns:    6,
			ConnMaxLifetime: 15 * time.Minute,
			ConnMaxIdleTime: 3 * time.Minute,
		},
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	app := flowpkg.New("pool-test", flowpkg.WithConfig(cfg))

	if app.DB() == nil {
		t.Fatal("DB() is nil after WithConfig with DatabaseURL set")
	}

	stats := app.DB().Stats()
	if stats.MaxOpenConnections != 12 {
		t.Errorf("MaxOpenConnections = %d, want 12", stats.MaxOpenConnections)
	}
}

// ---------------------------------------------------------------------------
// WithDBPool option — applies pool after WithBun
// ---------------------------------------------------------------------------

func TestWithDBPool_AppliesAfterWithBun(t *testing.T) {
	t.Parallel()

	adapter, err := orm.ConnectWithPool("file::memory:?cache=shared&mode=memory", orm.PoolConfig{})
	if err != nil {
		t.Fatalf("ConnectWithPool: %v", err)
	}

	app := flowpkg.New("pool-option-test",
		flowpkg.WithBun(adapter),
		flowpkg.WithDBPool(orm.PoolConfig{
			MaxOpenConns:    20,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
		}),
	)
	defer func() { _ = adapter.Close() }()

	if app.DB() == nil {
		t.Fatal("DB() is nil")
	}

	stats := app.DB().Stats()
	if stats.MaxOpenConnections != 20 {
		t.Errorf("MaxOpenConnections = %d, want 20", stats.MaxOpenConnections)
	}
}

// WithDBPool before any DB is opened must be a safe no-op.
func TestWithDBPool_NoDBIsNoop(t *testing.T) {
	t.Parallel()

	app := flowpkg.New("no-db-pool-test",
		flowpkg.WithDBPool(orm.PoolConfig{MaxOpenConns: 5}),
	)
	if app.DB() != nil {
		t.Fatal("DB() should be nil when no database was configured")
	}
}

// WithDBPool placed BEFORE WithConfig must be overridden by WithConfig's pool.
// (Order matters — last wins for each sql.DB setter.)
func TestWithDBPool_AfterWithConfig_WinsOverConfigPool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Env:             config.EnvDevelopment,
		Addr:            ":3000",
		LogLevel:        "info",
		DatabaseURL:     "sqlite://file::memory:?cache=shared",
		DBPool:          config.DBPoolConfig{MaxOpenConns: 10},
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	// WithDBPool placed AFTER WithConfig → its value should win.
	app := flowpkg.New("pool-order-test",
		flowpkg.WithConfig(cfg),
		flowpkg.WithDBPool(orm.PoolConfig{MaxOpenConns: 30}),
	)

	if app.DB() == nil {
		t.Fatal("DB() is nil")
	}

	stats := app.DB().Stats()
	if stats.MaxOpenConnections != 30 {
		t.Errorf("MaxOpenConnections = %d, want 30 (WithDBPool after WithConfig should win)", stats.MaxOpenConnections)
	}
}
