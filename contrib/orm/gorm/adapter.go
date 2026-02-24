package gormadapter

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// GormAdapter is a thin adapter that exposes the underlying *gorm.DB and
// *sql.DB for lifecycle operations similar to the repo's BunAdapter.
type GormAdapter struct {
	DB    *gorm.DB
	SQLDB *sql.DB
}

// ConnectSQLite opens a gorm DB using the sqlite driver and returns an adapter.
// dsn can be ":memory:" or a file path.
func ConnectSQLite(dsn string) (*GormAdapter, error) {
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open gorm sqlite: %w", err)
	}
	sqdb, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm DB(): %w", err)
	}
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
