package gormadapter

import (
	"context"
	"testing"

	"time"
)

func TestConnectSQLite_InMemory(t *testing.T) {
	g, err := ConnectSQLite(":memory:")
	if err != nil {
		t.Fatalf("ConnectSQLite: %v", err)
	}
	defer g.Close()

	// quick ping
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := g.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
