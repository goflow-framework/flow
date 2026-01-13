package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	orm "github.com/undiegomejia/flow/internal/orm"
	flow "github.com/undiegomejia/flow/pkg/flow"
	"github.com/uptrace/bun"
	_ "modernc.org/sqlite"
)

// Post is a simple model similar to what the generator produces. The generator
// will emit a similar struct in app/models when you run `flow generate model`.
type Post struct {
	flow.Model
	Title       string    `bun:"title" json:"title"`
	PublishedAt time.Time `bun:"published_at" json:"published_at"`
}

func main() {
	// ensure example data directory exists
	if err := os.MkdirAll(filepath.Dir("examples/bun_demo/db.sqlite"), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	ctx := context.Background()

	// connect to sqlite file for the example
	adapter, err := orm.Connect("file:examples/bun_demo/db.sqlite?_foreign_keys=1")
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer adapter.Close()

	app := flow.New("examples-bun-demo", flow.WithBun(adapter))

	// Auto-migrate the Post table (convenience for examples/tests)
	if err := flow.AutoMigrate(ctx, app, (*Post)(nil)); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	// insert a sample record using the Flow helper
	p := &Post{Title: "Hello from bun demo", PublishedAt: time.Now()}
	if err := flow.Insert(ctx, app, p); err != nil {
		log.Fatalf("insert: %v", err)
	}

	var got Post
	if err := flow.FindByPK(ctx, app, &got, p.ID); err != nil {
		log.Fatalf("find by pk: %v", err)
	}
	fmt.Printf("retrieved post: %#v\n", got)

	// demo RunInTx: insert a post inside a transaction
	if err := flow.RunInTx(ctx, app, func(ctx context.Context, tx *bun.Tx) error {
		p2 := &Post{Title: "tx-post", PublishedAt: time.Now()}
		if _, err := tx.NewInsert().Model(p2).Exec(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("transaction insert failed: %v", err)
	}

	var txGot Post
	if err := app.Bun().NewSelect().Model(&txGot).Where("title = ?", "tx-post").Scan(ctx); err != nil {
		log.Fatalf("select tx-post: %v", err)
	}
	fmt.Printf("tx inserted post: %#v\n", txGot)
}
