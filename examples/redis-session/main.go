package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	flow "github.com/dministrator/flow/pkg/flow"
	"github.com/redis/go-redis/v9"
)

func main() {
	// create app
	app := flow.New("examples-redis")

	// construct a Redis store and manager (example using localhost)
	opts := &redis.Options{Addr: "localhost:6379"}
	store := flow.NewRedisStore(opts, "examples:session:")
	rsm := flow.NewRedisSessionManager([]byte("super-secret-development-key"), "flow_session", store)

	// Register the redis-backed session middleware so handlers receive a session
	app.Use(rsm.Middleware())

	// You can still set router/views like other examples. For brevity we'll
	// just start the server and rely on the default ServeMux.
	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start error: %v\n", err)
		os.Exit(1)
	}

	// wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	_ = app.Shutdown(context.Background())
	// give a moment to flush logs
	time.Sleep(100 * time.Millisecond)
}
