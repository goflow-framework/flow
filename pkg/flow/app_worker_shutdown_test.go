package flow

import (
    "context"
    "testing"
    "time"

    miniredis "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/undiegomejia/flow/pkg/job"
)

// Test that App.Shutdown waits for in-flight job handlers to complete when
// the App created a bounded executor and StartWorker was used to start a
// Redis-backed job worker that submits handlers to that executor.
func TestAppShutdownWaitsForWorkerHandlers(t *testing.T) {
    m, err := miniredis.Run()
    if err != nil {
        t.Fatal(err)
    }
    defer m.Close()

    // Create an app that owns a bounded executor so Shutdown will call Shutdown
    // on the executor and wait for inflight tasks.
    a := New("test-app", WithBoundedExecutor(1, 4))

    // create queue and handler
    opts := &redis.Options{Addr: m.Addr()}
    q := job.NewRedisQueue(opts, "app-shutdown-test")
    defer q.Close()

    started := make(chan struct{})
    unblock := make(chan struct{})

    handlers := map[string]job.Handler{"hold": func(ctx context.Context, j *job.Job) error {
        // signal we've started, then block until unblocked
        close(started)
        <-unblock
        return nil
    }}

    // start the HTTP server so App.Shutdown will manage workers and executor.
    a.Addr = ":0"
    if err := a.Start(); err != nil {
        t.Fatalf("failed to start app: %v", err)
    }

    // start the worker (App will manage its lifecycle via StartWorker)
    w := a.StartWorker(q, handlers, job.WorkerOptions{PollInterval: 1 * time.Second, Concurrency: 1})
    _ = w

    // enqueue a job so the handler will be executed
    if err := q.Enqueue(context.Background(), &job.Job{Type: "hold"}); err != nil {
        t.Fatalf("enqueue: %v", err)
    }

    // wait for handler to start executing
    select {
    case <-started:
    case <-time.After(5 * time.Second):
        t.Fatal("handler did not start in time")
    }

    // now call Shutdown in background and ensure it blocks until we unblock the handler
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    done := make(chan error)
    go func() {
        done <- a.Shutdown(shutdownCtx)
    }()

    // give Shutdown a little time to begin; it should be blocked waiting for handler
    time.Sleep(200 * time.Millisecond)

    select {
    case err := <-done:
        t.Fatalf("Shutdown returned early: %v", err)
    default:
        // expected: still waiting
    }

    // now allow the handler to finish
    close(unblock)

    // Shutdown should complete shortly after
    select {
    case err := <-done:
        if err != nil {
            t.Fatalf("Shutdown returned error: %v", err)
        }
    case <-time.After(5 * time.Second):
        t.Fatal("Shutdown did not complete after handler finished")
    }
}
