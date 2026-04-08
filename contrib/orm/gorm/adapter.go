package gormadapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// GormAdapter is a thin adapter that exposes the underlying *gorm.DB and
// *sql.DB for lifecycle operations similar to the repo's BunAdapter.
type GormAdapter struct {
	DB    *gorm.DB
	SQLDB *sql.DB
}

// PoolConfig holds sql.DB connection-pool parameters for gorm adapters.
// It mirrors orm.PoolConfig so callers do not need to import the orm package.
//
// Apply it after opening a connection:
//
//	a, _ := gormadapter.ConnectSQLite(":memory:")
//	gormadapter.ApplyPool(a.SQLDB, gormadapter.PoolConfig{MaxOpenConns: 10})
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// ApplyPool configures connection-pool settings on db. Only non-zero values
// are applied so callers can supply a partially-filled PoolConfig without
// accidentally resetting fields they did not intend to change.
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

// ConnectSQLite opens a gorm DB using the sqlite driver and returns an adapter.
// dsn can be ":memory:" or a file path.
func ConnectSQLite(dsn string) (*GormAdapter, error) {
	return ConnectSQLiteWithPool(dsn, PoolConfig{})
}

// ConnectSQLiteWithPool opens a gorm SQLite DB and applies pool settings.
func ConnectSQLiteWithPool(dsn string, pool PoolConfig) (*GormAdapter, error) {
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open gorm sqlite: %w", err)
	}
	sqdb, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm DB(): %w", err)
	}
	ApplyPool(sqdb, pool)
	return &GormAdapter{DB: gdb, SQLDB: sqdb}, nil
}

// ConnectPostgres opens a gorm connection to Postgres using the provided DSN
// (for example: "host=... user=... password=... dbname=... sslmode=disable").
func ConnectPostgres(dsn string) (*GormAdapter, error) {
	return ConnectPostgresWithPool(dsn, PoolConfig{})
}

// ConnectPostgresWithPool opens a gorm Postgres DB and applies pool settings.
func ConnectPostgresWithPool(dsn string, pool PoolConfig) (*GormAdapter, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open gorm postgres: %w", err)
	}
	sqdb, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm DB(): %w", err)
	}
	ApplyPool(sqdb, pool)
	return &GormAdapter{DB: gdb, SQLDB: sqdb}, nil
}

// Close closes the underlying SQL DB connection.
func (g *GormAdapter) Close() error {
	if g == nil || g.SQLDB == nil {
		return nil
	}
	return g.SQLDB.Close()
}

// Ping checks connectivity.
func (g *GormAdapter) Ping(ctx context.Context) error {
	if g == nil || g.SQLDB == nil {
		return fmt.Errorf("gorm adapter: nil")
	}
	return g.SQLDB.PingContext(ctx)
}

// AutoMigrate runs gorm's AutoMigrate for the provided model types.
// It is a convenience wrapper for common migrations in examples/tests.
func (g *GormAdapter) AutoMigrate(models ...interface{}) error {
	if g == nil || g.DB == nil {
		return fmt.Errorf("gorm adapter: nil")
	}
	return g.DB.AutoMigrate(models...)
}

// WithTransaction runs fn inside a database transaction. It commits on nil
// error and rolls back otherwise. The provided fn receives a transactional
// *gorm.DB.
func (g *GormAdapter) WithTransaction(fn func(tx *gorm.DB) error) error {
	if g == nil || g.DB == nil {
		return fmt.Errorf("gorm adapter: nil")
	}
	return g.DB.Transaction(fn)
}
