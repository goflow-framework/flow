# Executor & Background Workers

This short note explains the executor ownership model in Flow and shows a small example combining `WithBoundedExecutor` and `App.StartWorker`.

Key points

- The minimal `Executor` interface lives in `pkg/exec` and is intentionally tiny:

  - `Submit(ctx context.Context, fn func(context.Context)) error`
  - `Shutdown(ctx context.Context) error`

- If you supply an executor to the App using `flow.WithExecutor(myExec)` then the App will use it but will not manage its lifecycle (start/shutdown) for you.
- If you want the App to create and manage a bounded executor, use `flow.WithBoundedExecutor(n, queueSize)`. The App will call `Shutdown(ctx)` on that executor during `App.Shutdown`.
- `App.StartWorker(queue, handlers, opts)` records the started worker and, if the App has an executor configured, will attach it to the worker so job handlers are dispatched through the executor.
- `App.Shutdown(ctx)` will stop recorded workers (calls `Stop()`), wait for their goroutines to exit, then call the configured executor shutdown (if the App created/owns one), and finally call tracer shutdown (if configured).

Minimal example

```go
package main

import (
  "context"
  "time"

  "github.com/goflow-framework/flow/pkg/flow"
  "github.com/goflow-framework/flow/pkg/job"
  "github.com/redis/go-redis/v9"
)

func main() {
  // Create an App that also creates a bounded executor with 4 workers and queue size 16.
  app := flow.New("my-app", flow.WithBoundedExecutor(4, 16))

  // create a RedisQueue (replace with real redis opts in prod)
  q := job.NewRedisQueue(&redis.Options{Addr: "localhost:6379"}, "my-queue")

  // handlers
  handlers := map[string]job.Handler{
    "send-email": func(ctx context.Context, j *job.Job) error {
      // do background work
      return nil
    },
  }

  // start a worker that uses the App's executor to dispatch handlers
  app.StartWorker(q, handlers, job.WorkerOptions{Concurrency: 2})

  // start http server etc. (omitted)

  // graceful shutdown example
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  _ = app.Shutdown(ctx)
}
```

Notes

- Use `WithExecutor` when you want to provide your own pooled/executor implementation and manage its lifecycle separately (for example, sharing an executor across multiple Apps or external services).
- Use `WithBoundedExecutor` when you want the App to own the executor and ensure it is shutdown during `App.Shutdown`.
- `StartWorker` is convenient for wiring `pkg/job` workers into the App lifecycle; workers started this way will be stopped and waited-on during `App.Shutdown`.

This design keeps background execution explicit and low-overhead while allowing apps to opt into centralized, bounded concurrency.
