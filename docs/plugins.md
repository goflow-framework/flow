# Plugins

Flow supports a small, explicit plugin model built around a canonical
`Plugin` interface (`pkg/flow.Plugin`) and a lightweight
`ServiceRegistry` attached to an `App` instance. The goal is to keep plugin
interactions explicit and testable while allowing simple runtime and
compile-time registration patterns.

Core concepts

- `flow.Plugin` — interface for lifecycle hooks (Init, Mount, Start, Stop).
- `flow.ServiceRegistry` — threadsafe registry for sharing typed services
  between plugins and application components (e.g. mailer, metrics client).
- `pkg/plugins` — a thin registry helper package used for compile-time
  registration (optional).

Registration patterns

1) Runtime registration (recommended)

This is the simplest pattern: your application (typically in `main`) calls
registration functions provided by a plugin package and explicitly wires
services into the `App`.

```go
package main

import (
    "context"
    "log"

    "github.com/undiegomejia/flow/pkg/flow"
    myplugin "github.com/yourorg/yourplugin"
)

func main() {
    app := flow.New("myapp")

    // plugin performs runtime registration (returns an error on misconfig)
    if err := myplugin.Register(app); err != nil {
        log.Fatalf("register plugin: %v", err)
    }

    // optional: start plugin background work if plugin implements Start
    // (see flow.Plugin.Start contract if used)

    // start app
    _ = app.Start()
    // ... run server loop, signal handling, etc.
    _ = app.Shutdown(context.Background())
}
```

2) Compile-time registration (init-time)

Packages can register themselves from `init()` using `pkg/plugins` so that
plugins are discovered by import side-effect. This is useful for first-party
plugins shipped with the repository. Prefer runtime registration for clarity
and easier testability.

Example plugin (compile-time registration)

```go
// pkg/plugins/example/example.go
package example

import (
    "context"

    "github.com/undiegomejia/flow/pkg/flow"
    "github.com/undiegomejia/flow/pkg/plugins"
)

type ExamplePlugin struct{}

func (p *ExamplePlugin) Name() string { return "example" }
func (p *ExamplePlugin) Version() string { return "v0.1.0" }
func (p *ExamplePlugin) Init(a *flow.App) error {
    // register a service for other components to consume
    _ = a.RegisterService("example.mailer", NewMailer())
    return nil
}
func (p *ExamplePlugin) Mount(a *flow.App) error {
    // register routes or middleware using app.Use / controllers
    return nil
}
func (p *ExamplePlugin) Middlewares() []flow.Middleware { return nil }
func (p *ExamplePlugin) Start(ctx context.Context) error { return nil }
func (p *ExamplePlugin) Stop(ctx context.Context) error { return nil }

func init() {
    plugins.Register(&ExamplePlugin{})
}
```

Bootstrapping and ApplyAll

If you use compile-time registration via `pkg/plugins`, call
`plugins.ApplyAll(app)` during application bootstrap to execute the
Init and Mount passes and register any middleware returned by plugins.

```go
// cmd/myapp/main.go
package main

import (
    "context"
    "log"

    "github.com/undiegomejia/flow/pkg/flow"
    "github.com/undiegomejia/flow/pkg/plugins"
)

func main() {
    app := flow.New("myapp", flow.WithDefaultMiddleware())

    // apply registered plugins (runs Init and Mount passes)
    if err := plugins.ApplyAll(app); err != nil {
        log.Fatalf("apply plugins: %v", err)
    }

    if err := app.Start(); err != nil {
        log.Fatalf("start app: %v", err)
    }
    // ... graceful shutdown handling omitted for brevity
    _ = app.Shutdown(context.Background())
}
```

Working with services

`ServiceRegistry` stores `interface{}` values keyed by name. Keep service
interfaces small and assert the concrete type at consumption points.

```go
// registering (in plugin Init or runtime Register)
_ = app.RegisterService("mailer", myMailer)

// retrieving (in handlers or other plugins)
svc, ok := app.GetService("mailer")
if ok {
    mailer := svc.(MyMailer) // type assert and use
    mailer.Send(...)
}
```

Testing plugins

- Unit test plugin Init/Mount using a plain `flow.New(...)` App instance and
  assert service registration or route/middleware effects.
- For compile-time plugins, tests may call `plugins.List()` and `plugins.Get()`
  or call `plugins.ApplyAll(app)` to exercise Init/Mount in order.

Notes & recommendations

- Prefer runtime registration for clarity and easier testing.
- Keep plugin surface area small — register services and mount routes or
  middleware; avoid mutating unrelated global state.
- Document plugin semantic versioning in its package and validate major
  compatibility at registration if needed.

