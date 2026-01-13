# Using Bun with Flow

This document shows a small workflow for using the generated Bun-compatible
models (produced by `flow generate model`) together with migrations and the
Flow `App` helpers.

Overview
- Generate a model with the CLI (example):

```sh
flow generate model Post title:string published_at:datetime
```

This will create `app/models/post.go` containing a struct tagged for Bun and
a timestamped migration under `db/migrate` (e.g. `20260108120000_create_posts.up.sql`).

Wiring Bun into your App

Below is a small example that shows how to attach a Bun adapter to the `App`,
run a development-time `AutoMigrate`, and perform basic DB operations.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    flow "github.com/undiegomejia/flow/pkg/flow"
    orm "github.com/undiegomejia/flow/internal/orm"
    _ "modernc.org/sqlite"
)

// Example model similar to what the generator emits. Generated models will
// include bun struct tags and can be used directly with the helpers below.
type Post struct {
    flow.Model
    Title       string    `bun:"title" json:"title"`
    PublishedAt time.Time `bun:"published_at" json:"published_at"`
}

func main() {
    ctx := context.Background()

    // Connect a Bun-backed DB (sqlite in this example). For production use a
    // persistent file or a different driver/DSN.
    adapter, err := orm.Connect("file:examples/bun_demo/db.sqlite?_foreign_keys=1")
    if err != nil {
        log.Fatalf("connect db: %v", err)
    }
    defer adapter.Close()

    // Create App and attach Bun
    app := flow.New("bun-demo", flow.WithBun(adapter))

    // AutoMigrate is a convenience for development/tests (it uses bun's
    // CreateTable helpers). The generator also emits SQL migrations in
    // db/migrate which you can run with the migration runner shown in the
    // docs below.
    if err := flow.AutoMigrate(ctx, app, (*Post)(nil)); err != nil {
        log.Fatalf("auto migrate: %v", err)
    }

    // Basic insert/select using the Flow helpers
    p := &Post{Title: "Hello Bun", PublishedAt: time.Now()}
    if err := flow.Insert(ctx, app, p); err != nil {
        log.Fatalf("insert via helper: %v", err)
    }

    var got Post
    if err := flow.FindByPK(ctx, app, &got, p.ID); err != nil {
        log.Fatalf("find by pk: %v", err)
    }
    fmt.Printf("got post: %#v\n", got)

    // Transactional example using RunInTx — useful for multiple related ops
    if err := flow.RunInTx(ctx, app, func(ctx context.Context, tx *bun.Tx) error {
        // use tx for fine-grained control inside the transaction
        p2 := &Post{Title: "InsideTx", PublishedAt: time.Now()}
        if _, err := tx.NewInsert().Model(p2).Exec(ctx); err != nil {
            return err
        }
        return nil
    }); err != nil {
        log.Fatalf("transaction failed: %v", err)
    }
}
```

Running generator-created migrations

The project includes a lightweight migration runner under `internal/migrations`.
After generating migrations with the generator you can apply them programmatically
or via a small CLI wrapper. Example usage:

```go
import (
    migrations "github.com/undiegomejia/flow/internal/migrations"
)

runner := migrations.MigrationRunner{}
if err := runner.ApplyAll("db/migrate", app.DB()); err != nil {
    // handle error
}
```

Notes
- `AutoMigrate` is convenient for development and tests but does not replace
  explicit SQL migrations for production deployments.
- The generator emits Bun-friendly struct tags and SQL migration files; after
  generating a model you can choose either to run the generated SQL migrations
  with the migration runner, or use `AutoMigrate` to create tables directly.

See `examples/bun_demo` for a runnable example.
