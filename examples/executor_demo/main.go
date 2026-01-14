package main

import (
    "context"
    "fmt"
    "log"
    "time"

    miniredis "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    flow "github.com/undiegomejia/flow/pkg/flow"
    "github.com/undiegomejia/flow/pkg/job"
)

// This example demonstrates creating an App with a bounded executor and
// wiring a Redis-backed Worker. It uses miniredis so it can run locally
// without external dependencies.
func main() {
    m, err := miniredis.Run()
    if err != nil {
        log.Fatalf("miniredis.Run: %v", err)
    }
    defer m.Close()

    // create app with bounded executor
    app := flow.New("executor-demo", flow.WithBoundedExecutor(2, 8))

    // create a redis queue
    opts := &redis.Options{Addr: m.Addr()}
    q := job.NewRedisQueue(opts, "demo")
    defer q.Close()

    // handler prints and sleeps briefly to simulate work
    handlers := map[string]job.Handler{"print": func(ctx context.Context, j *job.Job) error {
        fmt.Println("handler running:", j.Type)
        time.Sleep(500 * time.Millisecond)
        return nil
    }}

    // start a worker attached to the App's executor
    app.StartWorker(q, handlers, job.WorkerOptions{PollInterval: 1 * time.Second, Concurrency: 1})

    // enqueue a job
    if err := q.Enqueue(context.Background(), &job.Job{Type: "print"}); err != nil {
        log.Fatalf("enqueue: %v", err)
    }

    // wait a bit for processing then shutdown
    time.Sleep(2 * time.Second)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := app.Shutdown(ctx); err != nil {
        log.Fatalf("shutdown: %v", err)
    }
    fmt.Println("done")
}
