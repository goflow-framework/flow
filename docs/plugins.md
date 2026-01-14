# Plugins

Flow supports a small, explicit plugin model built around a canonical
`Plugin` interface and a lightweight `ServiceRegistry` attached to an
`App` instance. The goal is to keep plugin interactions explicit and
testable while allowing simple runtime registration patterns.

Core concepts

- `flow.Plugin` — interface for lifecycle hooks (Init, Mount, Start, Stop).
- `flow.ServiceRegistry` — threadsafe registry for sharing services between
  plugins and application components.

Registration patterns

1. Runtime registration (recommended)

```go
import "github.com/undiegomejia/flow/pkg/flow"

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
- Plugin: implement `plugins.Plugin` with `Name()`, `Init(*flow.App)`, `Mount(*flow.App)` and `Middlewares()`.
- Register: call `plugins.Register(p)` (typically from an `init()` in the plugin package).
- ApplyAll: the host application should call `plugins.ApplyAll(app)` during startup to run Init/Mount hooks and register any middleware returned by plugins.

Lifecycle
1. Init pass: `Init(a)` is called for all registered plugins in registration order. Use this to prepare shared resources, validate configuration or mutate the App.
2. Mount pass: `Mount(a)` is called for all plugins after Init. Use this to register routes (via `app.SetRouter` / controllers) or to call `app.Use(...)`.

Notes and recommendations
- Keep plugins small and focused. For most extensions you only need to provide middleware or mount a few routes.
- This plugin model is compile-time oriented; plugins are typically compiled into the application binary. A runtime plugin registry can be built on top of this API if needed later.

Example
1. Create a plugin package under `pkg/plugins/yourplugin`.
2. In its `init()` register the plugin.
3. In your `cmd/...` bootstrap code call `plugins.ApplyAll(app)` before `app.Start()`.
