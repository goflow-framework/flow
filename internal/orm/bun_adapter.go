// Package orm provides database adapter implementations for the Flow framework.
//
// BunAdapter wraps uptrace/bun and implements the api.ORM interface so
// application code can stay decoupled from the concrete ORM library.
//
// Two factories are provided:
//
//   - Connect(dsn)            – SQLite (pure-Go modernc driver, development default)
//   - NewPostgresAdapter(dsn) – PostgreSQL via pgx/v5 + bun pgdialect (production)
//
// DSN format for postgres:  postgres://user:pass@host:5432/dbname?sslmode=disable
// DSN format for sqlite:    file::memory:?cache=shared  or  /path/to/db.sqlite
package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	// pgx/v5 stdlib adapter – registers the "pgx" sql driver used by
	// NewPostgresAdapter. The blank import is intentional: the driver is
	// self-registering and has no exported API we need here.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PoolConfig holds sql.DB connection-pool parameters.  It intentionally
// mirrors config.DBPoolConfig so the orm package stays free of an import
// cycle with the config package.  flow.WithConfig copies the values across.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections to the database.
	// 0 means unlimited (sql.DB default). Non-zero is strongly recommended in
	// production to avoid connection storms.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of connections in the idle pool.
	// When 0 the sql.DB default of 2 is kept unchanged.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum time a connection may be reused.
	// 0 means connections are reused forever.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum time a connection may be idle before
	// being closed.  0 means idle connections are never closed.
	ConnMaxIdleTime time.Duration
}

// ApplyPool configures connection-pool settings on db.  Only non-zero values
// are applied so callers can pass a partially-filled PoolConfig without
// accidentally resetting fields they did not intend to change.
//
// Call this immediately after sql.Open, before handing the *sql.DB to bun or
// any other ORM layer.
func ApplyPool(db *sql.DB, cfg PoolConfig) {
	if db == nil {
		return
	}
	if cfg.MaxOpenConns != 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}

// BunAdapter is a thin wrapper around bun.DB that also satisfies the
// api.ORM interface. It exposes the underlying *bun.DB for callers that
// need richer query-builder access.
type BunAdapter struct {
	DB    *bun.DB
	SQLDB *sql.DB
}

// Connect opens a SQLite connection using the provided DSN and returns a
// BunAdapter. Suitable for development and tests.
// The caller is responsible for closing the returned adapter.
func Connect(dsn string) (*BunAdapter, error) {
	return ConnectWithPool(dsn, PoolConfig{})
}

// ConnectWithPool opens a SQLite connection and applies pool settings before
// wrapping the *sql.DB in a BunAdapter.
func ConnectWithPool(dsn string, pool PoolConfig) (*BunAdapter, error) {
	sqdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("orm: open sqlite: %w", err)
	}
	ApplyPool(sqdb, pool)
	db := bun.NewDB(sqdb, sqlitedialect.New())
	return &BunAdapter{DB: db, SQLDB: sqdb}, nil
}

// NewPostgresAdapter opens a PostgreSQL connection using the provided DSN
// and returns a BunAdapter backed by pgdialect.
//
// The DSN must be a libpq-compatible connection string or URL, e.g.:
//
//	postgres://user:pass@localhost:5432/mydb?sslmode=disable
//
// The caller is responsible for closing the returned adapter.
func NewPostgresAdapter(dsn string) (*BunAdapter, error) {
	return NewPostgresAdapterWithPool(dsn, PoolConfig{})
}

// NewPostgresAdapterWithPool opens a PostgreSQL connection and applies pool
// settings before wrapping the *sql.DB in a BunAdapter.
func NewPostgresAdapterWithPool(dsn string, pool PoolConfig) (*BunAdapter, error) {
	if dsn == "" {
		return nil, fmt.Errorf("orm: postgres DSN must not be empty")
	}
	sqdb, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("orm: open postgres: %w", err)
	}
	ApplyPool(sqdb, pool)
	db := bun.NewDB(sqdb, pgdialect.New())
	return &BunAdapter{DB: db, SQLDB: sqdb}, nil
}

// ConnectFromDSN is a convenience factory that inspects the DSN prefix and
// delegates to NewPostgresAdapterWithPool for postgres:// / postgresql:// URLs
// and to ConnectWithPool (SQLite) for everything else. This is the function
// used by flow.WithConfig when DatabaseURL is set.
func ConnectFromDSN(dsn string) (*BunAdapter, error) {
	return ConnectFromDSNWithPool(dsn, PoolConfig{})
}

// ConnectFromDSNWithPool is like ConnectFromDSN but applies pool settings.
func ConnectFromDSNWithPool(dsn string, pool PoolConfig) (*BunAdapter, error) {
	lower := strings.ToLower(dsn)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return NewPostgresAdapterWithPool(dsn, pool)
	}
	// Strip the sqlite:// scheme if present so the modernc driver receives
	// a plain file path or :memory: specifier.
	stripped := dsn
	if strings.HasPrefix(lower, "sqlite://") {
		stripped = dsn[9:]
	}
	return ConnectWithPool(stripped, pool)
}

// Close closes the underlying *sql.DB connection.
func (b *BunAdapter) Close() error {
	if b == nil || b.SQLDB == nil {
		return nil
	}
	return b.SQLDB.Close()
}

// Ping checks connectivity.
func (b *BunAdapter) Ping(ctx context.Context) error {
	if b == nil || b.SQLDB == nil {
		return fmt.Errorf("orm: bun adapter is nil")
	}
	return b.SQLDB.PingContext(ctx)
}

// ---------------------------------------------------------------------------
// api.ORM implementation
// ---------------------------------------------------------------------------
//
// These four methods satisfy the github.com/goflow-framework/flow/pkg/flow/api.ORM
// interface, letting application code stay decoupled from *bun.DB details.
// Each method delegates to bun's query builder so bun's dialect handling,
// column mapping, and hook system are fully preserved.

// Insert inserts dst into the database. dst must be a pointer to a bun
// model struct or a slice of model structs.
func (b *BunAdapter) Insert(ctx context.Context, dst interface{}) error {
	if b == nil || b.DB == nil {
		return fmt.Errorf("orm: adapter not initialised")
	}
	_, err := b.DB.NewInsert().Model(dst).Exec(ctx)
	return err
}

// Update updates dst in the database using its primary key.
func (b *BunAdapter) Update(ctx context.Context, dst interface{}) error {
	if b == nil || b.DB == nil {
		return fmt.Errorf("orm: adapter not initialised")
	}
	_, err := b.DB.NewUpdate().Model(dst).WherePK().Exec(ctx)
	return err
}

// Delete deletes dst from the database using its primary key.
func (b *BunAdapter) Delete(ctx context.Context, dst interface{}) error {
	if b == nil || b.DB == nil {
		return fmt.Errorf("orm: adapter not initialised")
	}
	_, err := b.DB.NewDelete().Model(dst).WherePK().Exec(ctx)
	return err
}

// FindByPK selects the row identified by id into dst. id is used as the
// WHERE condition against the model's primary key column.
func (b *BunAdapter) FindByPK(ctx context.Context, dst interface{}, id interface{}) error {
	if b == nil || b.DB == nil {
		return fmt.Errorf("orm: adapter not initialised")
	}
	return b.DB.NewSelect().Model(dst).Where("id = ?", id).Scan(ctx)
}
