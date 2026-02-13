package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyAndRollbackSQLite(t *testing.T) {
	td := t.TempDir()
	// create migrations dir
	migDir := filepath.Join(td, "db", "migrate")
	if err := os.MkdirAll(migDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// write up and down SQL
	up := filepath.Join(migDir, "20260101000000_create_tests.up.sql")
	down := filepath.Join(migDir, "20260101000000_create_tests.down.sql")
	if err := os.WriteFile(up, []byte("CREATE TABLE tests (id INTEGER PRIMARY KEY);"), 0o644); err != nil {
		t.Fatalf("write up: %v", err)
	}
	if err := os.WriteFile(down, []byte("DROP TABLE IF EXISTS tests;"), 0o644); err != nil {
		t.Fatalf("write down: %v", err)
	}

	// open sqlite db file
	dbPath := filepath.Join(td, "test.db")
	dsn := fmt.Sprintf("file:%s", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	runner := &MigrationRunner{}
	if err := runner.ApplyAll(migDir, db); err != nil {
		t.Fatalf("apply all: %v", err)
	}

	// verify table exists
	var cnt int
	if err := db.QueryRow("SELECT count(name) FROM sqlite_master WHERE type='table' AND name='tests'").Scan(&cnt); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected table tests to exist, got %d", cnt)
	}

	// verify migration tracking entry
	var mcnt int
	if err := db.QueryRow("SELECT count(1) FROM flow_migrations").Scan(&mcnt); err != nil {
		t.Fatalf("query flow_migrations: %v", err)
	}
	if mcnt != 1 {
		t.Fatalf("expected 1 applied migration, got %d", mcnt)
	}

	// re-run ApplyAll — should be idempotent and not add duplicate records
	if err := runner.ApplyAll(migDir, db); err != nil {
		t.Fatalf("apply all second time: %v", err)
	}
	if err := db.QueryRow("SELECT count(1) FROM flow_migrations").Scan(&mcnt); err != nil {
		t.Fatalf("query flow_migrations after reapply: %v", err)
	}
	if mcnt != 1 {
		t.Fatalf("expected 1 applied migration after reapply, got %d", mcnt)
	}

	// rollback
	if err := runner.RollbackLast(migDir, db); err != nil {
		t.Fatalf("rollback last: %v", err)
	}
	if err := db.QueryRow("SELECT count(name) FROM sqlite_master WHERE type='table' AND name='tests'").Scan(&cnt); err != nil {
		t.Fatalf("query sqlite_master after rollback: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected table tests to be dropped after rollback, got %d", cnt)
	}

	// ensure migration tracking entry removed
	if err := db.QueryRow("SELECT count(1) FROM flow_migrations").Scan(&mcnt); err != nil {
		t.Fatalf("query flow_migrations after rollback: %v", err)
	}
	if mcnt != 0 {
		t.Fatalf("expected 0 applied migrations after rollback, got %d", mcnt)
	}
}
