package flow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Test that PutContext (the router's defer) waits for spawned goroutines to
// complete. The handler spawns a goroutine that increments a counter after a
// short delay; when the HTTP client receives the response the counter should
// have been incremented because PutContext waited for the RequestGroup.
func TestPutContextWaitsForGoroutines(t *testing.T) {
	app := New("test-putcontext")
	r := NewRouter(app)

	var cnt int32
	r.Get("/inc", func(ctx *Context) {
		ctx.Go(func(cctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&cnt, 1)
			return nil
		})
		ctx.SetHeader("Content-Type", "text/plain")
		ctx.Status(http.StatusOK)
		_, _ = ctx.ResponseWriter().Write([]byte("ok"))
	})

	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/inc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if atomic.LoadInt32(&cnt) != 1 {
		t.Fatalf("expected counter to be 1 after request, got %d", cnt)
	}
}

// Test that a failing goroutine cancels the RequestGroup and prevents other
// goroutines from completing. The handler returns immediately and relies on
// deferred PutContext to wait; the counter should remain zero because the
// second goroutine should have observed cancellation.
func TestRequestGroupCancellationCancelsOtherGoroutines(t *testing.T) {
	app := New("test-cancel")
	r := NewRouter(app)

	var cnt int32
	r.Get("/cancel", func(ctx *Context) {
		ctx.Go(func(cctx context.Context) error {
			// this goroutine fails quickly and should cancel the group
			time.Sleep(10 * time.Millisecond)
			return fmt.Errorf("boom")
		})

		ctx.Go(func(cctx context.Context) error {
			select {
			case <-time.After(200 * time.Millisecond):
				atomic.AddInt32(&cnt, 1)
				return nil
			case <-cctx.Done():
				return nil
			}
		})

		ctx.SetHeader("Content-Type", "text/plain")
		ctx.Status(http.StatusOK)
		_, _ = ctx.ResponseWriter().Write([]byte("queued"))
	})

	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/cancel")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if atomic.LoadInt32(&cnt) != 0 {
		t.Fatalf("expected counter to be 0 after cancellation, got %d", cnt)
	}
}

// Test explicit Wait: handler spawns a goroutine that returns an error and
// then calls RequestGroup().Wait() and writes the error to the response.
func TestExplicitWaitSurfacesError(t *testing.T) {
	app := New("test-explicit-wait")
	r := NewRouter(app)

	r.Get("/explicit", func(ctx *Context) {
		ctx.Go(func(cctx context.Context) error {
			time.Sleep(20 * time.Millisecond)
			return fmt.Errorf("explicit-error")
		})
		if err := ctx.RequestGroup().Wait(); err != nil {
			if err := ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "something"}); err != nil {
				t.Fatalf("ctx.JSON failed: %v", err)
			}
			return
		}
		if err := ctx.JSON(http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			t.Fatalf("ctx.JSON failed: %v", err)
		}
	})

	srv := httptest.NewServer(r.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/explicit")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d (%s)", resp.StatusCode, string(body))
	}
}
