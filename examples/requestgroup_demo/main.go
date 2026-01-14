package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	flow "github.com/undiegomejia/flow/pkg/flow"
)

func main() {
	app := flow.New("requestgroup-demo")

	r := flow.NewRouter(app)

	// Register a simple handler that fans out work using ctx.Go. The
	// handler returns immediately; the framework will wait for spawned
	// goroutines to complete because PutContext calls RequestGroup().Wait().
	r.Get("/", func(ctx *flow.Context) {
		ctx.Go(func(cctx context.Context) error {
			log.Println("task1: started")
			select {
			case <-time.After(100 * time.Millisecond):
				log.Println("task1: done")
			case <-cctx.Done():
				log.Println("task1: cancelled")
			}
			return nil
		})

		ctx.Go(func(cctx context.Context) error {
			log.Println("task2: started")
			select {
			case <-time.After(200 * time.Millisecond):
				log.Println("task2: done")
			case <-cctx.Done():
				log.Println("task2: cancelled")
			}
			return nil
		})

		// Return to the framework immediately; PutContext will wait for
		// the spawned goroutines before allowing the Context to be
		// returned to the pool.
		ctx.SetHeader("Content-Type", "text/plain; charset=utf-8")
		ctx.Status(http.StatusOK)
		_, _ = ctx.W.Write([]byte("ok\n"))
	})

	// Endpoint that demonstrates cancellation propagation: one goroutine
	// returns an error which cancels the group's context; the other task
	// should observe cancellation and not complete its work.
	r.Get("/cancel", func(ctx *flow.Context) {
		var cancelledTask = 0

		ctx.Go(func(cctx context.Context) error {
			// erroring task
			time.Sleep(10 * time.Millisecond)
			return fmt.Errorf("boom")
		})

		ctx.Go(func(cctx context.Context) error {
			select {
			case <-time.After(200 * time.Millisecond):
				// would have completed
				cancelledTask = 1
				return nil
			case <-cctx.Done():
				// expected path when cancelled
				return nil
			}
		})

		ctx.SetHeader("Content-Type", "text/plain; charset=utf-8")
		ctx.Status(http.StatusOK)
		_, _ = ctx.W.Write([]byte("cancel endpoint queued\n"))
	})

	// Endpoint that explicitly waits for the RequestGroup and surfaces
	// any error returned by goroutines to the response.
	r.Get("/explicit-wait", func(ctx *flow.Context) {
		ctx.Go(func(cctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return fmt.Errorf("explicit error")
		})

		// explicitly wait and return the error to the client
		if err := ctx.RequestGroup().Wait(); err != nil {
			ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, map[string]string{"status": "no-error"})
	})

	app.SetRouter(r.Handler())

	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start error: %v\n", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	_ = app.Shutdown(context.Background())
}
