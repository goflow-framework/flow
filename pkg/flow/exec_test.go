package flow

import (
    "context"
    "sync/atomic"
    "testing"
    "time"
)

func TestBoundedExecutor_BasicConcurrency(t *testing.T) {
    be := NewBoundedExecutor(3, 5)
    var inFlight int32
    const total = 20

    for i := 0; i < total; i++ {
        if err := be.Submit(context.Background(), func(ctx context.Context) {
            atomic.AddInt32(&inFlight, 1)
            time.Sleep(10 * time.Millisecond)
            atomic.AddInt32(&inFlight, -1)
        }); err != nil {
            t.Fatalf("submit failed: %v", err)
        }
    }

    // shutdown and wait
    if err := be.Shutdown(context.Background()); err != nil {
        t.Fatalf("shutdown failed: %v", err)
    }
    if atomic.LoadInt32(&inFlight) != 0 {
        t.Fatalf("expected no inflight tasks after shutdown, got %d", inFlight)
    }
}

func TestBoundedExecutor_ContextCancel(t *testing.T) {
    be := NewBoundedExecutor(1, 1)
    // submit a long-running task that will occupy the single worker
    if err := be.Submit(context.Background(), func(ctx context.Context) { time.Sleep(50 * time.Millisecond) }); err != nil {
        t.Fatalf("initial submit failed: %v", err)
    }
    // submit another task to fill the queue buffer
    if err := be.Submit(context.Background(), func(ctx context.Context) { time.Sleep(50 * time.Millisecond) }); err != nil {
        t.Fatalf("second submit failed: %v", err)
    }
    // now submit with a canceled context; it should fail because worker busy and queue full
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
    defer cancel()
    if err := be.Submit(ctx, func(ctx context.Context) {}); err == nil {
        t.Fatalf("expected submit to fail due to context cancel or timeout")
    }
    _ = be.Shutdown(context.Background())
}
