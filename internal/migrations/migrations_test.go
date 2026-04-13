package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestRollbackLast_NoMigrations(t *testing.T) {
	td := t.TempDir()
	migDir := filepath.Join(td, "db", "migrate")
	if err := os.MkdirAll(migDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(td, "test.db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	runner := &MigrationRunner{}
	err = runner.RollbackLast(migDir, db)
	if err == nil {
		t.Fatal("expected error when no migrations applied, got nil")
	}
	// error must contain helpful context
	if !strings.Contains(err.Error(), "no applied migrations") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestApplyAll_IdempotentMultiple(t *testing.T) {
	td := t.TempDir()
	migDir := filepath.Join(td, "db", "migrate")
	if err := os.MkdirAll(migDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// two independent migrations
	files := map[string]string{
		"20260101000000_create_a.up.sql": "CREATE TABLE a (id INTEGER PRIMARY KEY);",
		"20260102000000_create_b.up.sql": "CREATE TABLE b (id INTEGER PRIMARY KEY);",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(migDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	dbPath := filepath.Join(td, "test.db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	runner := &MigrationRunner{}
	// first run applies both
	if err := runner.ApplyAll(migDir, db); err != nil {
		t.Fatalf("first ApplyAll: %v", err)
	}
	var cnt int
	if err := db.QueryRow("SELECT count(1) FROM flow_migrations").Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", cnt)
	}
	// second run is idempotent
	if err := runner.ApplyAll(migDir, db); err != nil {
		t.Fatalf("second ApplyAll: %v", err)
	}
	if err := db.QueryRow("SELECT count(1) FROM flow_migrations").Scan(&cnt); err != nil {
		t.Fatalf("count after reapply: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected still 2 after reapply, got %d", cnt)
	}
}

func TestPendingMigrations(t *testing.T) {
	td := t.TempDir()
	migDir := filepath.Join(td, "db", "migrate")
	if err := os.MkdirAll(migDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	names := []string{
		"20260101000000_alpha.up.sql",
		"20260102000000_beta.up.sql",
		"20260103000000_gamma.up.sql",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(migDir, n), []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	dbPath := filepath.Join(td, "test.db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	runner := &MigrationRunner{}
	// apply first migration only
	if err := runner.ApplyAll(migDir, db); err != nil {
		// SELECT 1 is valid; if we get an error apply each individually
		t.Fatalf("ApplyAll: %v", err)
	}
	// all three applied now; manually remove two from tracking to simulate pending
	if _, err := db.Exec("DELETE FROM flow_migrations WHERE name != '20260101000000_alpha'"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	pending, err := runner.PendingMigrations(migDir, db)
	if err != nil {
		t.Fatalf("PendingMigrations: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d: %v", len(pending), pending)
	}
}
