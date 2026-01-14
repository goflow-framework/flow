# Flow — A Minimal MVC Framework for Go

[![CI](https://github.com/undiegomejia/flow/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/undiegomejia/flow/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/undiegomejia/flow)](https://goreportcard.com/report/github.com/undiegomejia/flow)

> Flow is an opinionated, small, and developer-friendly MVC framework for Go. It provides a tiny routing DSL, controller/context helpers, a simple view manager with layouts/partials, cookie sessions, a migrations runner, and generators to scaffold controllers, models, views and migrations. Flow's design favors explicitness, testability and a pleasant developer loop — similar in spirit to Rails, but idiomatic Go.

This README gives a concise introduction, quickstart, and reference for the main building blocks so you (or contributors) can get started quickly.

## Highlights

- Small, dependency-free internal router with RESTful `Resources` helper.
- Public `pkg/flow` API with `Controller`, `Context`, `ViewManager`, and `App` bootstrap.
- Simple template loading with layout and partial support (conventions: `views/{controller}/{action}.html`, `views/layouts/*.html`, `views/partials/*`).
- Cookie-based sessions & flash helpers (lightweight, no external deps).
- Migration runner (timestamped up/down SQL) and CLI generator scaffolding (controllers, models, migrations).
- A PoC Bun ORM adapter and `AutoMigrate` helper; generator now emits Bun-tagged model structs and SQL migrations when fields are provided.
 - A PoC Bun ORM adapter and `AutoMigrate` helper; generator now emits Bun-tagged model structs and SQL migrations when fields are provided.
 - Basic ORM helpers and CRUD/transaction helpers exposed on `pkg/flow`: `Insert`, `Update`, `Delete`, `FindByPK`, `BeginTx` and `RunInTx` to simplify transactional patterns and generated-model usage.

## Quickstart (run the example)

The repository contains a small example app under `examples/simple`. To run it from a Linux environment (or WSL on Windows):

```bash
# from the repository root.
go run ./examples/simple
```

Open a browser or curl:

```bash
curl http://localhost:3000/users/1
```

The example demonstrates controllers and views (see `examples/simple/app/controllers` and `examples/simple/app/views`).

There is also a Bun ORM demo that demonstrates wiring the Bun adapter into the `App`, running `AutoMigrate`, and doing basic DB operations (Linux/WSL):

```bash
# from the repository root.
go run ./examples/bun_demo
```

See `docs/bun.md` for more details on using generated Bun models and migrations.

If you want a quick compile/run check for generated models, see `internal/generator/gen_compile_test.go` — it demonstrates generating a model into a temporary project, compiling a small program that uses the generated `Save`/`Delete` methods and running it to ensure end-to-end compilation.

## Development: serve --watch (hot-reload)

For a faster developer loop you can run the CLI in watch mode which restarts the server when source files change. The watcher lives in the CLI and spawns a child `go run ./cmd/flow serve --no-watch` process — the `--no-watch` flag is internal and prevents recursive watchers.

Basic usage (defaults watch current directory and common source files):

```bash
# run watcher and serve on :3000
flow serve --watch --addr :3000
```

Customize what to watch and what triggers a restart:

- `--watch-paths` — comma-separated list of directories to watch (default: `.`).
- `--watch-ignore` — comma-separated list of path names or simple patterns to ignore (default: `.git,vendor,node_modules`).
- `--watch-ext` — comma-separated list of file extensions that should trigger a restart (default: `.go,.tmpl,.html,.sql`). If empty, all file changes are considered.

Examples:

```bash
# watch only cmd and internal directories, ignore node_modules, and restart only on .go and .tmpl files
flow serve --watch --watch-paths cmd,internal --watch-ignore node_modules --watch-ext .go,.tmpl

# watch everything (no extension filter)
flow serve --watch --watch-ext ""
```

Notes:

- The watcher debounces rapid file events to avoid repeated restarts.
- Default ignore patterns include `.git`, `vendor` and `node_modules` to avoid noisy events.
- Use `--watch-ext` to reduce noise and speed up the loop (recommended).

## Enabling built-in middleware

Flow includes several small, useful middleware constructors (logging, request id,
timeout and simple metrics). You can enable them when constructing an `App`
using the provided functional options. The `WithDefaultMiddleware()` option
registers a sensible stack (Recovery, RequestID, Logging, Metrics).

Example — enable the default middleware stack:

```go
import (
	"time"
	"github.com/undiegomejia/flow/pkg/flow"
)

app := flow.New("my-app",
	flow.WithAddr(":3000"),
	flow.WithDefaultMiddleware(),
)
// start the app
_ = app.Start()
```

Example — customize middleware (add a per-request timeout):

```go
app := flow.New("my-app",
	flow.WithAddr(":3000"),
	flow.WithRequestID("X-Request-ID"),
	flow.WithLogging(),
	flow.WithTimeout(5*time.Second),
)
```

## Install & Tests

Make sure you have Go 1.20+ (project uses module mode). These commands assume a Linux environment — on Windows, run them inside WSL.

From the repository root:

```bash
# run all tests. From the repository root.
go test ./... -v

# build the project. From the repository root.
go build ./...
```

## Formatting

CI enforces `gofmt` formatting. To format the repository locally you can run the provided scripts:

- On WSL / Linux / macOS:

```bash
./scripts/format.sh
```

- On Windows PowerShell (run from the repository root):

```powershell
.\scripts\format.ps1
# add -Commit to automatically commit formatting changes
.\scripts\format.ps1 -Commit
```

The scripts run `gofmt -w .` (and `goimports -w .` if available) and print any remaining files that need formatting. After running, stage and commit the changes before pushing.

Developer commands (quick)

If you prefer running the commands directly instead of the scripts, here are common developer shortcuts you can use in WSL / Linux / macOS (run PowerShell variants on Windows as shown above).

```bash
# format all Go files in-place
gofmt -w .

# list files that would be reformatted (useful for CI validation)
gofmt -l .

# optionally run goimports if available to fix imports
goimports -w .

# run the full test suite (verbose)
go test ./... -v

# run tests for the public package only (faster during development)
go test ./pkg/flow -v

# run tests without cache (useful when editing test helpers)
go test ./... -count=1
```


## Key Concepts and Files

Below is a quick map of important packages and conventions to understand Flow's internals and how to use it.

- `internal/router` — a small HTTP router used by the framework. Supports parameterized routes (`/users/:id`), `Get/Post/...` helpers and `Resources(base, controller)` for RESTful wiring.
- `pkg/flow` — the public API surface used by application code:
	- `App` (in `pkg/flow/app.go`): application bootstrap (router, middleware, server lifecycle, Views and Sessions).
	- `Context` (in `pkg/flow/context.go`): request-scoped helper passed to controller actions (render, JSON, params, sessions, flash).
	- `Controller` (in `pkg/flow/controller.go`): base type and adapter helpers.
	- `ViewManager` (in `pkg/flow/view.go`): template loader, caching, layout and partial resolution.
	- `SessionManager` (in `pkg/flow/session.go`): cookie-based sessions and flash helpers.

### Router and Controllers (example)

Flow exposes a `NewRouter(app *flow.App)` constructor. Handlers accept `func(*flow.Context)` which simplifies controller code. Example:

```go
app := flow.New("my-app")
r := flow.NewRouter(app)

// register a Context-based handler
r.Get("/hello", func(ctx *flow.Context) {
		ctx.JSON(200, map[string]string{"hello": "world"})
})

// resources (RESTful routes)
users := NewUsersController(app) // implement flow.Resource
_ = r.Resources("users", users)

app.SetRouter(r.Handler())
```

RequestGroup helper
-------------------

Flow exposes a small structured-concurrency helper `RequestGroup` accessible
from a request `*flow.Context`. See `docs/requestgroup.md` for usage
examples, cancellation semantics and a small demo app under
`examples/requestgroup_demo`.

The `MakeResourceAdapter(app, res)` adapts a `flow.Resource` (methods that accept `*Context`) to the internal router.

See also: Executor & background workers (docs/executor.md) for guidance on running background tasks and wiring job workers to the App.

### Views and Templates

Flow uses `html/template` and a `ViewManager` with a small convention:

- Template directory: configurable via `NewViewManager("views")`.
- View lookup: `views/{controller}/{action}.html` (use `ViewManager.Render("users/show", data, ctx)`).
- Layouts: put shared layouts in `views/layouts/*.html` (layouts can call `{{ template "content" . }}` to insert the view content).
- Partials: put reusable fragments in `views/partials/*.html` and reference them in templates.

Example controller rendering:

```go
func (u *UsersController) Show(ctx *flow.Context) {
		data := map[string]interface{}{"Title": "User", "ID": ctx.Param("id")}
		_ = u.Render(ctx, "users/show", data)
}
```

### Sessions & Flash

Flow includes a minimal cookie-based signed session manager with helpers attached to `Context`:

- `ctx.Session()` returns the session for the request.
- `ctx.AddFlash(kind, message)` adds a flash message.
- `ctx.Flashes()` reads and clears flash messages.

The implementation is intentionally small and dependency-free to keep things portable and testable.

Accessing the session-stored user id

If you use the generated auth scaffolding, the middleware and controller helpers store the
authenticated user's id in the session. The generated middleware exposes a helper
`GetSessionUserID` that converts the stored session value into an int64 safely.

Example usage inside a handler (replace the middleware import path with your module path):

```go
import (
    middleware "github.com/your/module/path/app/middleware" // replace with your module
    flow "github.com/undiegomejia/flow/pkg/flow"
)

func (c *SomeController) Show(ctx *flow.Context) {
    if s := ctx.Session(); s != nil {
        if id, ok := middleware.GetSessionUserID(s); ok {
            // id is the authenticated user's ID (int64)
            _ = id // use it (e.g. fetch full user from DB if needed)
        }
    }
    // ...handler logic
}
```

### Migrations & Generators

There is an internal migrations runner that expects timestamped `*.up.sql` and `*.down.sql` files and runs them in a transaction. The CLI scaffolding provides generator helpers to create controllers, models, views and migrations.

Check `internal/migrations` and `internal/generator` for the implementation and templates.

New generator features:

- `flow generate model NAME [fields...]` — generate a model with optional field definitions (eg. `title:string published_at:datetime`). The generator will emit Bun struct tags (`bun:"field_name"`) and a migration SQL with the specified columns.
- `flow generate scaffold NAME [fields...]` — generate controller, model, views and migrations; fields are forwarded to the model generator.
- CLI: `cmd/flow` updated so `generate model` and `generate scaffold` accept variadic field args.
 - Generated models now include small convenience methods (`Save(ctx, app)` and `Delete(ctx, app)`) which call into the `flow` CRUD helpers. This makes generated code immediately usable with the Bun PoC adapter.
 - Generator integration tests: the repo contains CLI integration tests that build the CLI, run the generator into a temp project, and assert generated files and migration SQL. There's also a compile-and-run test that builds a tiny program against the generated model to ensure the generated code compiles and runs.

New generator flags:

- `--no-i18n` — when provided to scaffold/admin/auth generators, the generator will skip creating the minimal `app/i18n/en.yaml` translation file. This is useful for projects that don't need built-in i18n scaffolding.

See `docs/generator.md` for detailed generator flag documentation, field syntax and examples.

## Contributing

The project is organized to be easy to contribute to:

- Write small, focused tests (packages already include tests for router, migrations, generator templates).
- Follow repository conventions: controllers under `examples/<app>/app/controllers`, views under `examples/<app>/app/views`.
- Run `go test ./...` before opening a PR.

Suggested issues to start with (examples):

- Add more `ViewManager` FuncMap helpers (url helpers, formatters).
- Add flags and options to generators (force, db dialect).
- Improve CLI UX (cobra commands, descriptive messages).

## Project Status & Roadmap

The repository implements a working prototype with router, controllers, views (layouts/partials), sessions, migrations, and generators. Recent additions include:

- A Proof-of-Concept Bun ORM adapter (`internal/orm`) and `pkg/flow` helpers (`WithBun`, `SetBun`, `App.Bun()` and `AutoMigrate`).
- Generator upgrades to accept field lists and emit Bun-tagged models and migration SQL.
- CLI generator commands accept field arguments (`flow generate model NAME [fields...]`, `flow generate scaffold NAME [fields...]`).
- Documentation (`docs/bun.md`) and a runnable example (`examples/bun_demo`) demonstrating Bun usage.
 - Basic ORM helper surface added to `pkg/flow`: `Insert`, `Update`, `Delete`, `FindByPK`, `BeginTx` and `RunInTx` plus transaction helpers used by generated models.
 - Generator templates updated to include `Save` and `Delete` model methods so generated models are immediately usable.
 - Integration tests for generator CLI and a compile/run test ensure generated code compiles and behaves as expected.

Planned improvements:

- richer view helpers and FuncMap support,
- generator flags and options (eg. `--orm`, `--force`), and more robust field parsing (defaults, indexes, constraints),
- a `flow migrate` CLI wrapper for applying migrations,
- CI workflow and developer DX improvements (hot-reload integration),
- fuller documentation and more examples.

## Executor & Background Workers (brief)

Flow exposes a minimal Executor abstraction for running background work and integrating job workers with the App lifecycle. Key points:

- The shared Executor interface lives in `pkg/exec` and is intentionally tiny: `Submit(ctx, fn)` and `Shutdown(ctx)`.
- You can provide your own Executor to the App with `flow.WithExecutor(myExec)`; in this case the App will use it but will not manage its lifecycle (Start/Shutdown) unless you also provide an explicit shutdown function.
- To have the App create and manage a bounded executor for you, use `flow.WithBoundedExecutor(n, queueSize)` — the App will call `Shutdown(ctx)` on that executor during `App.Shutdown`.
- Use `App.StartWorker(queue, handlers, opts)` to start a `pkg/job` `Worker` that processes a `RedisQueue`. If the App has an Executor configured the worker will submit job handler executions to that executor; otherwise handlers run synchronously in worker goroutines.
- The App records workers started via `StartWorker` and will call `Stop()` on them and wait for them to finish during `App.Shutdown` so background work is drained before process exit.

This design keeps the runtime small and explicit: provide an executor when you want centralized, bounded concurrency and let the App manage shutdown for executors it created.

## License

This project is provided under an MIT-style license. Modify as appropriate for your needs.



