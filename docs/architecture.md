# Architecture: Plugin API and Versioning

This short document summarizes the plugin API surface and the compatibility rules used by Flow.

Plugin interface

The canonical plugin interface lives in `pkg/flow` and looks like:

- Name() string — unique plugin name used for registration and lookup.
- Version() string — semantic version (e.g. `v0.1.2` or `0.1.2`). The framework validates the major version.
- Init(app *flow.App) error — initialization step (synchronous). Use this to prepare shared resources and register services in the App's `ServiceRegistry`.
- Mount(app *flow.App) error — mount-time step (synchronous). Use this to register routes or middleware on the App.
- Middlewares() []flow.Middleware — optional middleware to be registered by the App when the plugin is mounted.
- Start(ctx context.Context) error — optional background work. The framework runs Start in a background goroutine when `App.Start()` is called. Start should return when the context is canceled.
- Stop(ctx context.Context) error — called during `App.Shutdown()` to allow the plugin to stop gracefully. Stop is invoked in reverse registration order.

Lifecycle and ordering

- Registration: Plugins can be registered at compile-time (package `init()` calling `pkg/plugins.Register(...)`) or at runtime with `app.RegisterPlugin(p)`.
- Init/Mount: The framework runs Init and Mount synchronously during registration (for `app.RegisterPlugin`) or when `plugins.ApplyAll(app)` migrates package-registered plugins into an App.
- Start: Background work starts when `App.Start()` is called. Plugin.Start gets a context that is canceled during `App.Shutdown()`.
- Stop: Plugins are stopped during `App.Shutdown()` in reverse registration order.

Versioning and compatibility

- The framework defines a constant `PluginAPIMajor` in `pkg/flow` representing the major version of the plugin API that this release of the framework supports.
- Plugins must expose a semantic version via `Version()`. The framework validates that the plugin major version equals `PluginAPIMajor`. If not, registration fails.
- This conservative policy avoids unexpected breakage caused by incompatible plugin API changes. To make breaking changes to the plugin API, bump `PluginAPIMajor` and follow the repository's governance/migration guidance.

Best practices

- Prefer runtime registration (`app.RegisterPlugin`) in user applications for clarity and control.
- Keep plugin interfaces small — expose services in App.ServiceRegistry instead of tightly coupling components.
- Make Start implementations responsive to context cancellation and avoid long, uninterruptible work.

More details and examples

See `docs/plugins.md` and `examples/plugins` for runnable examples showing runtime registration and package-level registration migration (using `plugins.ApplyAll(app)`).
