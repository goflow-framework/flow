package job

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestEnqueueDequeueRoundtrip(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	opts := &redis.Options{Addr: m.Addr()}
	q := NewRedisQueue(opts, "testjob")
	defer q.Close()

	j := &Job{Type: "hello", Payload: json.RawMessage(`{"n":1}`)}
	if err := q.Enqueue(context.Background(), j); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	popped, err := q.popImmediate(context.Background())
	if err != nil {
		t.Fatalf("pop: %v", err)
	}
	if popped == nil {
		t.Fatalf("expected job, got nil")
	}
	if popped.Type != "hello" {
		t.Fatalf("expected type hello, got %s", popped.Type)
	}
}

func TestWorkerRetriesAndBackoff(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	opts := &redis.Options{Addr: m.Addr()}
	q := NewRedisQueue(opts, "testjob2")
	defer q.Close()

	// handler that fails the first two times then succeeds
	var calls int32
	handler := func(ctx context.Context, j *Job) error {
		v := atomic.AddInt32(&calls, 1)
		if v <= 2 {
			return &temporaryError{"try again"}
		}
		// on success, signal via setting job payload to include success flag
		return nil
	}

	handlers := map[string]Handler{"retry": handler}
	w := NewWorker(q, handlers, WorkerOptions{PollInterval: 50 * time.Millisecond, BackoffBase: 10 * time.Millisecond, JitterMillis: 0, Concurrency: 1})

	j := &Job{Type: "retry", MaxAttempts: 5}
	if err := q.Enqueue(context.Background(), j); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// run worker; it will stop when ctx is done
	go func() { _ = w.Start(ctx) }()

	// wait until handler called at least 3 times (2 failures + 1 success)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&calls) >= 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("handler not called enough times: calls=%d", atomic.LoadInt32(&calls))
}

// temporaryError is a sentinel error type used to indicate transient failures.
type temporaryError struct{ s string }

func (t *temporaryError) Error() string { return t.s }

func (t *temporaryError) Temporary() bool { return true }

// sanity helper for test visibility of internal popImmediate (in same package)
func TestDelayedScheduling(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	opts := &redis.Options{Addr: m.Addr()}
	q := NewRedisQueue(opts, "testjob3")
	defer q.Close()

	j := &Job{Type: "later", MaxAttempts: 1}
	if err := q.EnqueueAt(context.Background(), j, time.Now().Add(300*time.Millisecond)); err != nil {
		t.Fatalf("enqueue at: %v", err)
	}

	// moveDue should not move it immediately
	if err := q.moveDue(context.Background()); err != nil {
		t.Fatalf("moveDue: %v", err)
	}
	p, _ := q.popImmediate(context.Background())
	if p != nil {
		t.Fatalf("expected no immediate job, got one")
	}

	// after waiting, moveDue should move it
	time.Sleep(350 * time.Millisecond)
	if err := q.moveDue(context.Background()); err != nil {
		t.Fatalf("moveDue: %v", err)
	}
	p2, _ := q.popImmediate(context.Background())
	if p2 == nil || p2.Type != "later" {
		t.Fatalf("expected delayed job to be available, got %+v", p2)
	}
}

func TestWorkerConcurrency(t *testing.T) {
	resetMetrics()
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	opts := &redis.Options{Addr: m.Addr()}
	q := NewRedisQueue(opts, "testjob-conc")
	defer q.Close()

	const total = 50
	for i := 0; i < total; i++ {
		j := &Job{ID: fmt.Sprintf("j-%d", i), Type: "noop", MaxAttempts: 1}
		if err := q.Enqueue(context.Background(), j); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	// handler that does minimal work
	handlers := map[string]Handler{"noop": func(ctx context.Context, j *Job) error { return nil }}
	w := NewWorker(q, handlers, WorkerOptions{PollInterval: 50 * time.Millisecond, BackoffBase: 10 * time.Millisecond, JitterMillis: 0, Concurrency: 5})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = w.Start(ctx) }()

	// wait until processed count reaches total
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if Metrics().Processed >= total {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("not all jobs processed: processed=%d", Metrics().Processed)
}

func TestWorkerLargeVolume(t *testing.T) {
	resetMetrics()
	m, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	opts := &redis.Options{Addr: m.Addr()}
	q := NewRedisQueue(opts, "testjob-large")
	defer q.Close()

	const total = 200
	for i := 0; i < total; i++ {
		j := &Job{ID: fmt.Sprintf("jl-%d", i), Type: "noop", MaxAttempts: 1}
		if err := q.Enqueue(context.Background(), j); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	handlers := map[string]Handler{"noop": func(ctx context.Context, j *Job) error { return nil }}
	w := NewWorker(q, handlers, WorkerOptions{PollInterval: 20 * time.Millisecond, BackoffBase: 10 * time.Millisecond, JitterMillis: 0, Concurrency: 10})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() { _ = w.Start(ctx) }()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if Metrics().Processed >= total {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("not all large-volume jobs processed: processed=%d", Metrics().Processed)
}
