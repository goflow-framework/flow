# Redis session example

This example shows how to wire Flow's Redis-backed session manager into a minimal application.

Prerequisites
- Go 1.18+ (module mode enabled)
- A running Redis instance reachable at `localhost:6379` (the example uses this address by default)

Quick start

From the repository root run:

```powershell
go run ./examples/redis-session
```

Or from the example directory:

```powershell
cd examples/redis-session
go run main.go
```

What the example does
- Creates a `flow.App` and a Redis store using `github.com/redis/go-redis/v9`.
- Constructs a `RedisSessionManager` with a development secret key and registers its middleware on the app.
- Starts an HTTP server on the default address `:3000`.

Notes
- The example uses a hard-coded development secret (`super-secret-development-key`). Replace this with a strong, random key for any non-development usage.
- To use a Redis server on a different host/port, edit `main.go` and change the `redis.Options{Addr: "host:port"}` value.
- The example doesn't register custom routes — add handlers or set `app.SetRouter(...)` to test session persistence across requests.

Testing with Docker (optional)

If you don't have Redis locally, start one with Docker:

```powershell
docker run --rm -p 6379:6379 redis:7
```

Once Redis is running, start the example and observe the logs. Stop the server with Ctrl+C.
