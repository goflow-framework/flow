RequestGroup and ContextPool
===========================

Overview
--------
This document describes two small runtime helpers in the `flow` framework
that reduce per-request allocations and provide structured concurrency for
request-scoped goroutines:

- RequestGroup: a lightweight structured-concurrency primitive that anchors
  goroutines to a request context and captures the first non-nil error.
- ContextPool: a `sync.Pool` wrapper that reuses `*flow.Context` instances
  across requests to avoid small per-request allocations.

Why use them
------------
Per-request allocations add up under load. Small objects like `flow.Context`
and transient goroutine bookkeeping for short-lived background work can
cause noticeable pressure on the garbage collector. Reusing `Context`
instances via a pool eliminates allocations/op for the hot path and yields
lower latency and fewer GC cycles.

RequestGroup makes it simple to run several goroutines during request
processing and wait for their completion before finishing the request. It
also cancels the group's context when one goroutine returns an error so
other goroutines can stop early.

Quick usage
-----------

1. Obtain a `*flow.Context` from the pool (framework adapters already do
   this for you when using `Router` / `Controller` helpers):

```
func myHandler(ctx *flow.Context) {
    // do work...
}

// using Router: commons wrappers call NewContext/PutContext automatically
r := flow.NewRouter(app)
r.Get("/", func(c *flow.Context) { /* ... */ })
```

2. Spawn request-scoped goroutines with `RequestGroup`:

```
// Inside a handler with *flow.Context available (ctx)
rg := ctx.RequestGroup()
rg.Go(func(c context.Context) error {
    // use c to observe cancellation and do background work
    return nil
})
// when handler returns, framework's PutContext waits for rg.Wait()
```

Benchmarks (summary)
--------------------
Ran locally with `go test ./pkg/flow -bench . -benchmem`.

- NewContext (no pool): ~37 ns/op, 48 B/op, 1 alloc/op
- NewContext (package or app pool): ~16 ns/op, 0 B/op, 0 alloc/op
- Context with RequestGroup (pool enabled): ~196 ns/op, 176 B/op, 3 allocs/op
- Context with RequestGroup (pool disabled): ~212 ns/op, 224 B/op, 4 allocs/op

These numbers will vary by CPU and OS but show clear allocation reduction
when using pooling (0 allocs/op vs 1 alloc/op in the microbenchmarks).

Notes
-----
- The pool is enabled by default (`UseContextPool = true`) and each `App`
  is initialized with a per-App `ContextPool` (`New()` sets `ctxPool`).
- The framework's router and controller adapters already use `NewContext`
  and `PutContext` so most apps get pooling automatically. Only code that
  constructs `&flow.Context{}` manually needs to switch to `NewContext`.
- `PutContext` waits for any request-scoped goroutines spawned via
  `RequestGroup` to finish before returning the instance to the pool.

Recommendations
---------------
- Prefer `NewContext` and `PutContext` in any custom handlers or adapters.
- Use `RequestGroup` for short-lived request-scoped goroutines and avoid
  long-running background work — use App-level executors for that.
- If you observe weird leaks or tests that rely on object identity, you
  can disable pooling (`flow.UseContextPool = false`) for deterministic
  behavior during debugging.
