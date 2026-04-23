package orm_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/goflow-framework/flow/internal/orm"

	_ "modernc.org/sqlite" // register the "sqlite" driver
)

// openSQLite is a helper that opens an in-memory sqlite DB directly via
// database/sql so we can inspect stats after ApplyPool.
func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// ApplyPool
// ---------------------------------------------------------------------------

func TestApplyPool_SetsMaxOpenConns(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	orm.ApplyPool(db, orm.PoolConfig{MaxOpenConns: 17})
	if got := db.Stats().MaxOpenConnections; got != 17 {
		t.Errorf("MaxOpenConnections = %d, want 17", got)
	}
}

func TestApplyPool_SetsMaxIdleConns(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	orm.ApplyPool(db, orm.PoolConfig{MaxOpenConns: 10, MaxIdleConns: 7})
	// sql.DB doesn't expose MaxIdleConns directly, but we verify no panic
	// and that MaxOpenConns was also applied.
	if got := db.Stats().MaxOpenConnections; got != 10 {
		t.Errorf("MaxOpenConnections = %d, want 10", got)
	}
}

func TestApplyPool_ZeroFieldsLeaveDefaults(t *testing.T) {
	t.Parallel()
	db := openSQLite(t)
	// sql.DB default MaxOpenConns is 0 (unlimited); verify we don't change it
	// when PoolConfig is entirely zero.
	orm.ApplyPool(db, orm.PoolConfig{})
	if got := db.Stats().MaxOpenConnections; got != 0 {
		t.Errorf("zero PoolConfig should not change MaxOpenConns (want 0), got %d", got)
	}
}

func TestApplyPool_NilDBIsNoop(t *testing.T) {
	t.Parallel()
	// Must not panic.
	orm.ApplyPool(nil, orm.PoolConfig{MaxOpenConns: 5})
}

// ---------------------------------------------------------------------------
// ConnectWithPool (SQLite)
// ---------------------------------------------------------------------------

func TestConnectWithPool_AppliesPool(t *testing.T) {
	t.Parallel()
	pool := orm.PoolConfig{
		MaxOpenConns:    8,
		MaxIdleConns:    4,
		ConnMaxLifetime: 20 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	adapter, err := orm.ConnectWithPool("file::memory:?cache=shared&mode=memory", pool)
	if err != nil {
		t.Fatalf("ConnectWithPool: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	stats := adapter.SQLDB.Stats()
	if stats.MaxOpenConnections != 8 {
		t.Errorf("MaxOpenConnections = %d, want 8", stats.MaxOpenConnections)
	}
}

// ---------------------------------------------------------------------------
// Connect (no pool config) — backwards compatibility
// ---------------------------------------------------------------------------

func TestConnect_BackwardsCompat(t *testing.T) {
	t.Parallel()
	adapter, err := orm.Connect("file::memory:?cache=shared&mode=memory")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	if adapter.DB == nil {
		t.Fatal("adapter.DB is nil")
	}
	if adapter.SQLDB == nil {
		t.Fatal("adapter.SQLDB is nil")
	}
}

// ---------------------------------------------------------------------------
// ConnectFromDSN / ConnectFromDSNWithPool — routing
// ---------------------------------------------------------------------------

func TestConnectFromDSN_SQLite(t *testing.T) {
	t.Parallel()
	adapter, err := orm.ConnectFromDSN("sqlite://file::memory:?cache=shared&mode=memory")
	if err != nil {
		t.Fatalf("ConnectFromDSN(sqlite://): %v", err)
	}
	defer func() { _ = adapter.Close() }()

	if adapter.SQLDB == nil {
		t.Fatal("SQLDB is nil")
	}
}

func TestConnectFromDSNWithPool_SQLite(t *testing.T) {
	t.Parallel()
	pool := orm.PoolConfig{MaxOpenConns: 3}
	adapter, err := orm.ConnectFromDSNWithPool("sqlite://file::memory:?cache=shared&mode=memory", pool)
	if err != nil {
		t.Fatalf("ConnectFromDSNWithPool: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	if got := adapter.SQLDB.Stats().MaxOpenConnections; got != 3 {
		t.Errorf("MaxOpenConnections = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// NewPostgresAdapterWithPool — DSN error path (no live DB needed)
// ---------------------------------------------------------------------------

func TestNewPostgresAdapterWithPool_EmptyDSN(t *testing.T) {
	t.Parallel()
	_, err := orm.NewPostgresAdapterWithPool("", orm.PoolConfig{})
	if err == nil {
		t.Fatal("expected error for empty postgres DSN, got nil")
	}
}
