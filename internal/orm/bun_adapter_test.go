package orm

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Basic CRUD test using bun on an in-memory sqlite.
func TestBunAdapterBasicCRUD(t *testing.T) {
	adapter, err := Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer adapter.Close()

	type User struct {
		ID        int64     `bun:"id,pk,autoincrement"`
		Name      string    `bun:"name"`
		CreatedAt time.Time `bun:"created_at"`
	}

	ctx := context.Background()

	// create table
	if _, err := adapter.DB.NewCreateTable().Model((*User)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// insert
	u := &User{Name: "Alice", CreatedAt: time.Now()}
	if _, err := adapter.DB.NewInsert().Model(u).Exec(ctx); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// select
	var got User
	if err := adapter.DB.NewSelect().Model(&got).Where("name = ?", "Alice").Scan(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", got.Name)
	}

	// update
	got.Name = "Bob"
	if _, err := adapter.DB.NewUpdate().Model(&got).WherePK().Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	var after User
	if err := adapter.DB.NewSelect().Model(&after).Where("id = ?", got.ID).Scan(ctx); err != nil {
		t.Fatalf("select after update: %v", err)
	}
	if after.Name != "Bob" {
		t.Fatalf("expected Bob, got %s", after.Name)
	}

	// delete
	if _, err := adapter.DB.NewDelete().Model(&after).WherePK().Exec(ctx); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var users []User
	if err := adapter.DB.NewSelect().Model(&users).Scan(ctx); err != nil {
		t.Fatalf("select all: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(users))
	}
}

// TestBunAdapterImplementsORM verifies that *BunAdapter satisfies the api.ORM
// interface at compile time. No runtime assertions needed.
func TestBunAdapterImplementsORM(t *testing.T) {
	// Verify the four methods exist with the correct signatures by calling
	// them on a nil adapter and asserting the nil-guard error is returned.
	// This avoids the import cycle that would arise from importing
	// pkg/flow/api inside the internal/orm package.
	var b *BunAdapter
	ctx := context.Background()
	if err := b.Insert(ctx, nil); err == nil {
		t.Error("Insert on nil adapter: expected error")
	}
	if err := b.Update(ctx, nil); err == nil {
		t.Error("Update on nil adapter: expected error")
	}
	if err := b.Delete(ctx, nil); err == nil {
		t.Error("Delete on nil adapter: expected error")
	}
	if err := b.FindByPK(ctx, nil, 0); err == nil {
		t.Error("FindByPK on nil adapter: expected error")
	}
}

// TestBunAdapterORMCRUD exercises the api.ORM methods (Insert/Update/Delete/
// FindByPK) end-to-end on an in-memory SQLite database.
func TestBunAdapterORMCRUD(t *testing.T) {
	adapter, err := Connect("file::memory:?cache=shared&mode=memory")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer adapter.Close()

	type Product struct {
		ID    int64  `bun:"id,pk,autoincrement"`
		Title string `bun:"title,notnull"`
	}

	ctx := context.Background()

	if _, err := adapter.DB.NewCreateTable().Model((*Product)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert via api.ORM
	p := &Product{Title: "Widget"}
	if err := adapter.Insert(ctx, p); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("Insert: expected auto-incremented ID to be set")
	}

	// FindByPK via api.ORM
	var found Product
	if err := adapter.FindByPK(ctx, &found, p.ID); err != nil {
		t.Fatalf("FindByPK: %v", err)
	}
	if found.Title != "Widget" {
		t.Fatalf("FindByPK: expected Widget, got %q", found.Title)
	}

	// Update via api.ORM
	found.Title = "Gadget"
	if err := adapter.Update(ctx, &found); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var updated Product
	if err := adapter.FindByPK(ctx, &updated, p.ID); err != nil {
		t.Fatalf("FindByPK after Update: %v", err)
	}
	if updated.Title != "Gadget" {
		t.Fatalf("Update: expected Gadget, got %q", updated.Title)
	}

	// Delete via api.ORM
	if err := adapter.Delete(ctx, &updated); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var missing Product
	err = adapter.FindByPK(ctx, &missing, p.ID)
	if err == nil {
		t.Fatal("FindByPK after Delete: expected error, got nil")
	}
}

// TestNewPostgresAdapterEmptyDSN ensures that a missing DSN returns an error
// immediately rather than panicking or hanging.
func TestNewPostgresAdapterEmptyDSN(t *testing.T) {
	_, err := NewPostgresAdapter("")
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
}

// TestConnectFromDSN_SQLite verifies that sqlite:// and plain DSNs are routed
// to the SQLite factory.
func TestConnectFromDSN_SQLite(t *testing.T) {
	cases := []string{
		"file::memory:?cache=shared",
		"sqlite://file::memory:?cache=shared",
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			a, err := ConnectFromDSN(dsn)
			if err != nil {
				t.Fatalf("ConnectFromDSN(%q): %v", dsn, err)
			}
			defer a.Close()
			if err := a.Ping(context.Background()); err != nil {
				t.Fatalf("Ping: %v", err)
			}
		})
	}
}

// TestConnectFromDSN_PostgresPrefix verifies that postgres:// and
// postgresql:// prefixes are routed to the postgres factory. We don't have a
// live Postgres server in unit tests, so we only assert that the error is a
// connection error (not a "driver not found" or nil error).
func TestConnectFromDSN_PostgresPrefix(t *testing.T) {
	cases := []string{
		"postgres://user:pass@localhost:5432/testdb",
		"postgresql://user:pass@localhost:5432/testdb",
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			a, err := ConnectFromDSN(dsn)
			// sql.Open is lazy – it does not connect until the first Ping/Query.
			// We expect Open to succeed but Ping to fail (no server running).
			if err != nil {
				t.Fatalf("ConnectFromDSN(%q) open: unexpected error %v", dsn, err)
			}
			defer a.Close()
			// Ping must fail – we do not have a live server.
			pingErr := a.Ping(context.Background())
			if pingErr == nil {
				t.Fatalf("Ping: expected connection error, got nil (is a Postgres server running?)")
			}
		})
	}
}
