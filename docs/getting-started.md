# Getting Started with Flow

This guide walks you through creating a new Flow application from scratch — scaffolding the project, running your first migration, writing a controller, and starting the development server.

**Prerequisites:** Go 1.24 or later installed and `$GOPATH/bin` (or `~/go/bin`) on your `PATH`.

---

## 1. Install the CLI

```bash
go install github.com/goflow-framework/flow/cmd/flow@latest
```

Verify the install:

```bash
flow --version
# flow version vX.Y.Z
```

---

## 2. Scaffold a new application

```bash
flow new myapp
```

This creates the following layout:

```
myapp/
  cmd/myapp/main.go      # application entry-point
  internal/              # private application packages (controllers, models, …)
  db/migrations/         # timestamped SQL migration files
  go.mod                 # module definition
  .env.example           # documented environment variables
  Makefile               # common developer tasks (run, build, test, migrate, rollback)
```

The generated `main.go` wires a minimal `flow.App` with the default middleware stack:

```go
package main

import (
    "log"
    "github.com/goflow-framework/flow/pkg/flow"
)

func main() {
    app := flow.New("myapp",
        flow.WithDefaultMiddleware(),
    )
    if err := app.Run(); err != nil {
        log.Fatal(err)
    }
}
```

---

## 3. Configure the environment

```bash
cd myapp
cp .env.example .env
```

Edit `.env` — the defaults work out of the box for local development:

```dotenv
# Runtime environment: development | test | production
FLOW_ENV=development

# HTTP listen address
FLOW_ADDR=:3000

# Primary database DSN (postgres or sqlite)
DATABASE_URL=sqlite://db/development.db

# Session signing secret – set a random 32+ char string in production
# FLOW_SECRET_KEY_BASE=changeme
```

Then install dependencies:

```bash
go mod tidy
```

---

## 4. Run your first migration

Create a migration file:

```bash
# Naming convention: <timestamp>_<description>.up.sql / .down.sql
touch db/migrations/001_create_users.up.sql
touch db/migrations/001_create_users.down.sql
```

`001_create_users.up.sql`:
```sql
CREATE TABLE users (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL,
    email TEXT   NOT NULL UNIQUE
);
```

`001_create_users.down.sql`:
```sql
DROP TABLE users;
```

Apply it:

```bash
flow db migrate --dir db/migrations
# or via the Makefile shorthand:
make migrate
```

Check the current migration state:

```bash
flow db status --dir db/migrations
```

Roll back the last migration:

```bash
flow db rollback --dir db/migrations
# or:
make rollback
```

By default the `--dsn` flag is not required — Flow reads `DATABASE_URL` from the environment.  
To override: `flow db migrate --dsn postgres://user:pass@localhost/myapp`.

---

## 5. Write a controller

Create `internal/controllers/users.go`:

```go
package controllers

import (
    "net/http"

    "github.com/goflow-framework/flow/pkg/flow"
)

type UsersController struct{}

// SignupForm is the input struct for POST /users.
// BindForm fills it from the request body; Validate enforces the rules.
type SignupForm struct {
    Name     string `form:"name"     validate:"required"`
    Email    string `form:"email"    validate:"required,email"`
    Password string `form:"password" validate:"required,min=8"`
}

// Create handles POST /users
func (uc *UsersController) Create(ctx *flow.Context) {
    var f SignupForm
    if err := ctx.BindForm(&f); err != nil {
        ctx.Error(http.StatusBadRequest, err.Error())
        return
    }
    if err := ctx.Validate(&f); err != nil {
        ctx.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
        return
    }
    // … insert into DB, redirect, etc.
    ctx.JSON(http.StatusCreated, map[string]string{"name": f.Name, "email": f.Email})
}
```

Key context helpers used above:

| Method | Description |
|---|---|
| `ctx.BindForm(&dst)` | Decode `application/x-www-form-urlencoded` body into a struct using `` `form:"name"` `` tags |
| `ctx.BindQuery(&dst)` | Decode URL query parameters into a struct (same tag convention) |
| `ctx.Validate(&dst)` | Run `go-playground/validator` rules declared in `` `validate:"..."` `` tags |
| `ctx.JSON(code, v)` | Write a JSON response |
| `ctx.Render(name, data)` | Render an HTML template via the configured `ViewManager` |
| `ctx.Param("id")` | Read a URL path parameter |
| `ctx.Error(code, msg)` | Write a plain-text error response |

Wire the controller into `main.go`:

```go
import (
    "github.com/goflow-framework/flow/internal/controllers"
    "github.com/goflow-framework/flow/internal/router"
    flowrouter "github.com/goflow-framework/flow/internal/router"
)

func main() {
    app := flow.New("myapp", flow.WithDefaultMiddleware())

    r := flowrouter.New()
    uc := &controllers.UsersController{}
    r.Post("/users", func(ctx *flow.Context) { uc.Create(ctx) })

    app.SetRouter(r)
    log.Fatal(app.Run())
}
```

---

## 6. Start the development server

```bash
flow serve --addr :3000
# or:
make run
```

Test it:

```bash
curl -s http://localhost:3000/health
# {"status":"ok"}
```

Optional flags for `flow serve`:

| Flag | Default | Description |
|---|---|---|
| `--addr` | `:3000` | HTTP listen address |
| `--metrics-addr` | *(off)* | Expose Prometheus `/metrics` (e.g. `:9090`) |
| `--trace-stdout` | `false` | Print OpenTelemetry spans to stdout |
| `--otlp-endpoint` | *(off)* | OTLP gRPC collector address (e.g. `otel-collector:4317`) |
| `--service-name` | `flow` | `service.name` attribute sent to the tracer |

---

## 7. Scaffold controllers, models, and migrations

The `flow generate` sub-command creates boilerplate so you don't write it by hand.

```bash
# Scaffold a controller
flow generate controller posts

# Scaffold a model with fields (emits a Bun-tagged struct + SQL migration)
flow generate model post title:string body:text published:bool

# Scaffold a blank migration
flow generate migration add_index_to_users_email
```

Generated files appear under `internal/` and `db/migrations/` — inspect and adjust before applying.

---

## 8. Use the ORM (optional)

Flow ships a [Bun](https://bun.uptrace.dev/) adapter. When `DATABASE_URL` is set, `flow.WithConfig` auto-opens it:

```go
cfg, _ := flow.LoadConfig()
app := flow.New("myapp",
    flow.WithConfig(cfg),        // opens DB automatically from cfg.DatabaseURL
    flow.WithDefaultMiddleware(),
)

// Inside a controller, reach the *bun.DB via the adapter:
// app.Bun() returns *bun.DB
```

Run auto-migrations for registered models:

```go
flow.AutoMigrate(app.Bun(), (*models.User)(nil))
```

See [`docs/bun.md`](bun.md) for full ORM usage including transactions (`RunInTx`), generated model helpers (`Save`, `Delete`), and the Postgres factory (`orm.NewPostgresAdapter`).

---

## 9. Run the test suite

```bash
# Run all tests
go test ./...

# With coverage (same filter used by CI)
go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... \
  | xargs go test -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out   # open in browser
```

The CI coverage gate requires **≥ 50% aggregate coverage**. `internal/generator` tests run in an isolated job and are excluded from the broad run.

---

## Next steps

| Topic | Where to look |
|---|---|
| Plugin system | [`docs/plugins.md`](plugins.md) — `Plugin` interface, `ServiceRegistry`, lifecycle hooks |
| Security defaults | [`docs/security.md`](security.md) — CSRF, secure headers, `WithSecureDefaults` |
| ORM & Bun | [`docs/bun.md`](bun.md) — adapters, auto-migrate, transactions |
| Observability | [`docs/observability.md`](observability.md) — OpenTelemetry tracing, Prometheus metrics |
| Request groups | [`docs/requestgroup.md`](requestgroup.md) — scoped goroutines with `ctx.Go(fn)` |
| Context pool | [`docs/context_pool.md`](context_pool.md) — per-request pool tuning |
| Architecture | [`docs/architecture.md`](architecture.md) — plugin API versioning, `PluginAPIMajor` |
| Good first issues | [`docs/GOOD_FIRST_ISSUES.md`](GOOD_FIRST_ISSUES.md) — contribution ideas |
