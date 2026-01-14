package flow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestGroupRunsAll(t *testing.T) {
	rg := NewRequestGroup(context.Background())
	var cnt int32
	const n = 5

	for i := 0; i < n; i++ {
		rg.Go(func(ctx context.Context) error {
			atomic.AddInt32(&cnt, 1)
			return nil
		})
	}

	if err := rg.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&cnt) != n {
		t.Fatalf("expected %d tasks run, got %d", n, cnt)
	}
}

func TestRequestGroupCancelsOnError(t *testing.T) {
	rg := NewRequestGroup(context.Background())

	want := errors.New("boom")

	// goroutine that returns an error after a short delay
	rg.Go(func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return want
	})

	var sawCancelled int32
	// goroutine that should observe the cancellation
	rg.Go(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			atomic.StoreInt32(&sawCancelled, 1)
			return nil
		case <-time.After(300 * time.Millisecond):
			return nil
		}
	})

	err := rg.Wait()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != want.Error() {
		t.Fatalf("expected error %v, got %v", want, err)
	}
	if atomic.LoadInt32(&sawCancelled) == 0 {
		t.Fatalf("expected second goroutine to observe cancellation")
	}
}
