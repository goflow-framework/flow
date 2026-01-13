package flow

import (
	"context"
	"fmt"
	"testing"
	"time"

	orm "github.com/undiegomejia/flow/internal/orm"
	"github.com/uptrace/bun"
	_ "modernc.org/sqlite"
)

func TestAppSetBunAndAutoMigrate(t *testing.T) {
	adapter, err := orm.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("connect bun: %v", err)
	}
	defer adapter.Close()

	app := New("bun-test", WithBun(adapter))

	type Item struct {
		ID        int64     `bun:"id,pk,autoincrement"`
		Name      string    `bun:"name"`
		CreatedAt time.Time `bun:"created_at"`
	}

	ctx := context.Background()
	if err := AutoMigrate(ctx, app, (*Item)(nil)); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	db := DB(app)
	if db == nil {
		t.Fatalf("expected bun DB on app")
	}

	// basic insert/select to ensure bun is usable via App
	it := &Item{Name: "alpha", CreatedAt: time.Now()}
	if _, err := db.NewInsert().Model(it).Exec(ctx); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got Item
	if err := db.NewSelect().Model(&got).Where("name = ?", "alpha").Scan(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got.Name != "alpha" {
		t.Fatalf("expected alpha, got %s", got.Name)
	}
}

func TestRunInTxRollback(t *testing.T) {
	adapter, err := orm.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("connect bun: %v", err)
	}
	defer adapter.Close()

	app := New("bun-test-tx", WithBun(adapter))

	type ItemTx struct {
		ID   int64  `bun:"id,pk,autoincrement"`
		Name string `bun:"name"`
	}

	ctx := context.Background()
	if err := AutoMigrate(ctx, app, (*ItemTx)(nil)); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// run in tx and force an error to trigger rollback
	err = RunInTx(ctx, app, func(ctx context.Context, tx *bun.Tx) error {
		it := &ItemTx{Name: "tx-test"}
		if _, err := tx.NewInsert().Model(it).Exec(ctx); err != nil {
			return err
		}
		return fmt.Errorf("force rollback")
	})
	if err == nil {
		t.Fatalf("expected error from transaction function")
	}

	// ensure the record was not committed
	var got ItemTx
	err = app.Bun().NewSelect().Model(&got).Where("name = ?", "tx-test").Scan(ctx)
	if err == nil {
		t.Fatalf("expected no rows after rollback, found: %#v", got)
	}
}

func TestCRUDOperations(t *testing.T) {
	adapter, err := orm.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("connect bun: %v", err)
	}
	defer adapter.Close()

	app := New("bun-test-crud", WithBun(adapter))

	type ItemCrud struct {
		ID   int64  `bun:"id,pk,autoincrement"`
		Name string `bun:"name"`
	}

	ctx := context.Background()
	if err := AutoMigrate(ctx, app, (*ItemCrud)(nil)); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// Insert using helper
	it := &ItemCrud{Name: "crud-insert"}
	if err := Insert(ctx, app, it); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if it.ID == 0 {
		t.Fatalf("expected inserted ID to be set")
	}

	// FindByPK
	var got ItemCrud
	if err := FindByPK(ctx, app, &got, it.ID); err != nil {
		t.Fatalf("FindByPK failed: %v", err)
	}
	if got.Name != "crud-insert" {
		t.Fatalf("unexpected name: %s", got.Name)
	}

	// Update
	it.Name = "crud-updated"
	if err := Update(ctx, app, it); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	var got2 ItemCrud
	if err := FindByPK(ctx, app, &got2, it.ID); err != nil {
		t.Fatalf("FindByPK after update failed: %v", err)
	}
	if got2.Name != "crud-updated" {
		t.Fatalf("update not applied, got: %s", got2.Name)
	}

	// Delete
	if err := Delete(ctx, app, it); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	var after ItemCrud
	if err := FindByPK(ctx, app, &after, it.ID); err == nil {
		t.Fatalf("expected not found after delete, got: %#v", after)
	}

	// RunInTx commit path
	if err := RunInTx(ctx, app, func(ctx context.Context, tx *bun.Tx) error {
		it2 := &ItemCrud{Name: "tx-commit"}
		if _, err := tx.NewInsert().Model(it2).Exec(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("RunInTx commit failed: %v", err)
	}
	var committed ItemCrud
	if err := app.Bun().NewSelect().Model(&committed).Where("name = ?", "tx-commit").Scan(ctx); err != nil {
		t.Fatalf("expected committed row, scan failed: %v", err)
	}
}
