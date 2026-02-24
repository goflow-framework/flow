package gormadapter

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"
)

// This test requires a Postgres instance and reads the DSN from PG_DSN.
// In CI the job provides PG_DSN; locally you can set PG_DSN when running tests.
func TestConnectPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN not set; skipping Postgres integration test")
	}
	g, err := ConnectPostgres(dsn)
	if err != nil {
		t.Fatalf("ConnectPostgres: %v", err)
	}
	defer g.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// simple transaction test
	err = g.WithTransaction(func(tx *gorm.DB) error {
		// no-op transaction to ensure TX path works
		return nil
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
}
