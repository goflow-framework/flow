package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MigrationRunner runs timestamped SQL migrations stored in a directory.
// Migration files follow the naming convention:
//
//	20260108120000_create_users.up.sql
//	20260108120000_create_users.down.sql
//
// ApplyAll executes all .up.sql files in ascending timestamp order.
type MigrationRunner struct{}

// ApplyAll applies all up migrations found in dir using the provided db.
// This version tracks applied migrations in a `flow_migrations` table so
// repeated runs are idempotent.
func (m *MigrationRunner) ApplyAll(dir string, db *sql.DB) error {
	// ensure migrations table exists
	if err := m.ensureTable(db); err != nil {
		return err
	}

	ups, err := m.collect(dir, ".up.sql")
	if err != nil {
		return err
	}
	sort.Strings(ups)
	for _, p := range ups {
		base := strings.TrimSuffix(filepath.Base(p), ".up.sql")
		applied, err := m.isApplied(db, base)
		if err != nil {
			return err
		}
		if applied {
			// skip already applied
			continue
		}
		if err := m.execFile(db, p); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(p), err)
		}
		if err := m.markApplied(db, base); err != nil {
			return fmt.Errorf("mark applied %s: %w", base, err)
		}
	}
	return nil
}

// RollbackLast finds the latest applied migration and executes its down SQL.
func (m *MigrationRunner) RollbackLast(dir string, db *sql.DB) error {
	// ensure migrations table exists
	if err := m.ensureTable(db); err != nil {
		return err
	}

	// find last applied migration
	var base string
	err := db.QueryRow("SELECT name FROM flow_migrations ORDER BY applied_at DESC LIMIT 1").Scan(&base)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no applied migrations found in %s", dir)
		}
		return err
	}

	// construct down file path
	downPath := filepath.Join(dir, base+".down.sql")
	if _, err := os.Stat(downPath); err != nil {
		return fmt.Errorf("down migration not found for %s: %w", base, err)
	}
	if err := m.execFile(db, downPath); err != nil {
		return fmt.Errorf("rollback %s: %w", filepath.Base(downPath), err)
	}
	if err := m.unmarkApplied(db, base); err != nil {
		return fmt.Errorf("unmark applied %s: %w", base, err)
	}
	return nil
}

// collect returns absolute paths of files in dir that end with suffix.
func (m *MigrationRunner) collect(dir, suffix string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("migrations directory not found: %s", dir)
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, suffix) {
			out = append(out, filepath.Join(dir, name))
		}
	}
	return out, nil
}

func (m *MigrationRunner) execFile(db *sql.DB, path string) error {
	// #nosec G304 -- migration SQL files are trusted developer artifacts within the project
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sqlText := string(b)
	// Execute in a transaction for safety
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(sqlText); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ApplySingle runs a single migration file (convenience)
func (m *MigrationRunner) ApplySingle(path string, db *sql.DB) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}
	// execute and mark applied if it's an up migration
	if err := m.execFile(db, path); err != nil {
		return err
	}
	if strings.HasSuffix(path, ".up.sql") {
		if err := m.ensureTable(db); err != nil {
			return err
		}
		base := strings.TrimSuffix(filepath.Base(path), ".up.sql")
		if err := m.markApplied(db, base); err != nil {
			return err
		}
	}
	return nil
}

// ListMigrations returns file names of migrations (both up and down) in dir.
func (m *MigrationRunner) ListMigrations(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".up.sql") || strings.HasSuffix(d.Name(), ".down.sql") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// ensureTable creates the migrations tracking table if it does not exist.
func (m *MigrationRunner) ensureTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS flow_migrations (
        name TEXT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`)
	return err
}

// isApplied checks if a migration (by base name) is already applied.
func (m *MigrationRunner) isApplied(db *sql.DB, base string) (bool, error) {
	var cnt int
	err := db.QueryRow("SELECT count(1) FROM flow_migrations WHERE name = ?", base).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// markApplied records a migration as applied.
func (m *MigrationRunner) markApplied(db *sql.DB, base string) error {
	_, err := db.Exec("INSERT INTO flow_migrations(name) VALUES (?)", base)
	return err
}

// unmarkApplied removes a migration record (used on rollback).
func (m *MigrationRunner) unmarkApplied(db *sql.DB, base string) error {
	_, err := db.Exec("DELETE FROM flow_migrations WHERE name = ?", base)
	return err
}

// AppliedMigrations returns the names of applied migrations in applied order.
func (m *MigrationRunner) AppliedMigrations(db *sql.DB) ([]string, error) {
	if err := m.ensureTable(db); err != nil {
		return nil, err
	}
	rows, err := db.Query("SELECT name FROM flow_migrations ORDER BY applied_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// PendingMigrations returns the list of up migration base names that are not yet applied.
func (m *MigrationRunner) PendingMigrations(dir string, db *sql.DB) ([]string, error) {
	if err := m.ensureTable(db); err != nil {
		return nil, err
	}
	ups, err := m.collect(dir, ".up.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(ups)
	var out []string
	for _, p := range ups {
		base := strings.TrimSuffix(filepath.Base(p), ".up.sql")
		applied, err := m.isApplied(db, base)
		if err != nil {
			return nil, err
		}
		if !applied {
			out = append(out, base)
		}
	}
	return out, nil
}
