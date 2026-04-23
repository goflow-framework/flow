Plugin ecosystem and best practices
=================================

This document describes conventions and best-practices for writing plugins
for the Flow framework. Plugins are first-class, supported extension points
that should be small, well-documented, and easy to test.

Plugin shape
------------
- Keep plugins under `contrib/` with a clear package path, for example
  `contrib/plugin/mailer`.
- Provide a small, well-documented public interface for the plugin. For
  example, a mailer plugin should expose a `Mailer` interface with a
  `Send(to, subject, body string) error` method.
- Offer one or more adapters in the package (SMTP, console/dummy, third-party)
  and a `NewXXXAdapter` constructor returning the adapter type that implements
  the public interface.

Lifecycle and wiring
--------------------
- Plugins should be instantiated during application startup and passed to the
  App via functional options or set in the App's ServiceRegistry.
- Prefer explicit wiring over global singletons. Example:

```go
m := mailer.NewSMTPAdapter("smtp.example.com:587", "user", "pass")
app := flow.New("my-app", flow.WithMailer(m))
```

Configuration
-------------
- Keep configuration simple: accept a small set of parameters in constructors
  and provide helper functions to load from environment variables when
  appropriate.
- Avoid parsing complex config formats inside plugins; accept a typed config
  struct where possible to make testing easier.

Testing
-------
- Provide unit tests for plugin logic; include a small `mock` implementation
  (or interfaces) to help clients test their integration without network
  access.

Documentation
-------------
- Each plugin must have a README.md with usage examples and a short API
  reference. Add an example under `examples/` when useful.
# Plugins

Flow supports a small, explicit plugin model built around a canonical
`Plugin` interface and a lightweight `ServiceRegistry` attached to an
`App` instance. The goal is to keep plugin interactions explicit and
testable while allowing simple runtime registration patterns.

Core concepts

  plugins and application components.

Registration patterns

1. Runtime registration (recommended)

```go
import "github.com/goflow-framework/flow/pkg/flow"

app := flow.New("myapp")

// register a named service
_ = app.RegisterService("mailer", myMailer)

// later
svc, ok := app.GetService("mailer")
if ok {
    mailer := svc.(MyMailer)
    // use mailer
}
```

2. Compile-time registration (use sparingly)

Plugins may register themselves during package `init()` by calling a
registration helper in `pkg/plugins`. This is useful for first-party
plugins distributed with the repo. Runtime registration (explicit calls in
`main`) is preferred for clarity.

ServiceRegistry contract

Services are stored as `interface{}`; consumers should document and assert
expected concrete types. Keep service interfaces small and well documented.

Example plugins

 See `examples/plugins/simple` for a minimal plugin that registers a service
and demonstrates `App.RegisterService` usage.

Security note

Plugins run with the same process privileges as the app. Do not load
untrusted third-party plugins in production without additional isolation.
# Plugins

Flow plugins provide a lightweight way to extend an application at boot
time. The minimal plugin API is intentionally small and aims for compile-time
registration (plugins call `plugins.Register(...)` from init()).

Key concepts

Lifecycle
1. Init pass: `Init(a)` is called for all registered plugins in registration order. Use this to prepare shared resources, validate configuration or mutate the App.
2. Mount pass: `Mount(a)` is called for all plugins after Init. Use this to register routes (via `app.SetRouter` / controllers) or to call `app.Use(...)`.

Notes & recommendations

- Prefer runtime registration for clarity and easier testing.
- Keep plugin surface area small — register services and mount routes or
  middleware; avoid mutating unrelated global state.
- Document plugin semantic versioning in its package and validate major
  compatibility at registration if needed.

Runtime example

Below is a short runtime registration example. A runnable example is
available at `examples/plugins/runtime` in the repository.

```go
// main.go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goflow-framework/flow/pkg/flow"
)

type myPlugin struct{}
func (p *myPlugin) Name() string { return "my-plugin" }
func (p *myPlugin) Version() string { return "v0.0.0" }
func (p *myPlugin) Init(a *flow.App) error { return a.RegisterService("my.key", "value") }
func (p *myPlugin) Mount(a *flow.App) error { return nil }
func (p *myPlugin) Middlewares() []flow.Middleware { return nil }
func (p *myPlugin) Start(ctx context.Context) error { return nil }
func (p *myPlugin) Stop(ctx context.Context) error { return nil }

func main() {
    app := flow.New("runtime-example")
    // Runtime registration: preferred. Registers and runs Init/Mount immediately
    // for this App instance only.
    if err := app.RegisterPlugin(&myPlugin{}); err != nil {
        log.Fatalf("register plugin: %v", err)
    }

    if v, ok := app.GetService("my.key"); ok {
        fmt.Println(v)
    }
    _ = app.Shutdown(context.Background())
}
```
