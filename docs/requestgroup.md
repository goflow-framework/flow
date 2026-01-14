RequestGroup and ctx.Go usage
=============================

Flow provides a small structured-concurrency helper for request-scoped
goroutines: `RequestGroup`. It's available from a `*flow.Context` via
`ctx.RequestGroup()` and there's a convenience helper `ctx.Go(fn)` which
spawns a goroutine attached to the current request.

Key semantics
- `ctx.RequestGroup()` is lazily created and is anchored to the
  request's `context.Context` (it observes client cancellations and
  deadlines).
- `ctx.Go(fn)` runs `fn(ctx)` in the request group where `ctx` is the
  group's derived context. If `fn` returns a non-nil error the group
  cancels the context, signalling other goroutines to stop.
- The framework waits for the RequestGroup when returning the Context to
  the pool: `PutContext` calls `RequestGroup().Wait()` if the group was
  created. This means the common handler pattern used by the router
  (`ctx := NewContext(...); defer PutContext(ctx); handler(ctx)`) is
  safe to use with `ctx.Go(...)` — the framework ensures spawned
  goroutines complete before the Context is reused.

Example

```go
func handler(ctx *flow.Context) {
    // spawn background work bound to the request
    ctx.Go(func(cctx context.Context) error {
        // do work; cctx will be cancelled if another goroutine fails
        return nil
    })

    // return immediately — the framework's deferred PutContext will wait
    // for the spawned goroutines before the Context is returned to the
    // pool.
    ctx.JSON(200, map[string]string{"status": "ok"})
}
```

Notes
- `RequestGroup.Wait()` returns aggregated errors; `PutContext` ignores
  those errors because there is no good place to surface them at the
  framework level. If your application needs to observe returned errors
  explicitly call `ctx.RequestGroup().Wait()` inside the handler instead
  of relying on automatic waiting.
RequestGroup and ctx.Go usage
=============================

Flow provides a small structured-concurrency helper for request-scoped
goroutines: `RequestGroup`. It's available from a `*flow.Context` via
`ctx.RequestGroup()` and there's a convenience helper `ctx.Go(fn)` which
spawns a goroutine attached to the current request.

Key semantics
- `ctx.RequestGroup()` is lazily created and is anchored to the
  request's `context.Context` (it observes client cancellations and
  deadlines).
- `ctx.Go(fn)` runs `fn(ctx)` in the request group where `ctx` is the
  group's derived context. If `fn` returns a non-nil error the group
  cancels the context, signalling other goroutines to stop.
- The framework waits for the RequestGroup when returning the Context to
  the pool: `PutContext` calls `RequestGroup().Wait()` if the group was
  created. This means the common handler pattern used by the router
  (`ctx := NewContext(...); defer PutContext(ctx); handler(ctx)`) is
  safe to use with `ctx.Go(...)` — the framework ensures spawned
  goroutines complete before the Context is reused.

Example

```go
func handler(ctx *flow.Context) {
    // spawn background work bound to the request
    ctx.Go(func(cctx context.Context) error {
        // do work; cctx will be cancelled if another goroutine fails
        return nil
    })

    // return immediately — the framework's deferred PutContext will wait
    // for the spawned goroutines before the Context is returned to the
    // pool.
    ctx.JSON(200, map[string]string{"status": "ok"})
}
```

Notes
- `RequestGroup.Wait()` returns aggregated errors; `PutContext` ignores
  those errors because there is no good place to surface them at the
  framework level. If your application needs to observe returned errors
  explicitly call `ctx.RequestGroup().Wait()` inside the handler instead
  of relying on automatic waiting.
