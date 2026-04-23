package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	flow "github.com/goflow-framework/flow/pkg/flow"
	"github.com/goflow-framework/flow/pkg/plugins"
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
	// register CSRF middleware
	app.Use(flow.CSRFMiddleware())

	// simple example handlers to demonstrate reading/writing sessions
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s := flow.FromContext(r.Context())
		if s == nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		v, _ := s.Get("count")
		cnt, _ := v.(float64) // json numbers decode to float64
		fmt.Fprintf(w, "count=%v\ncsrf=%v", cnt, flow.CSRFToken(r))
	})

	mux.HandleFunc("/inc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s := flow.FromContext(r.Context())
		if s == nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		v, _ := s.Get("count")
		cnt, _ := v.(float64)
		cnt++
		_ = s.Set("count", cnt)
		fmt.Fprintf(w, "ok count=%v", cnt)
	})

	app.SetRouter(mux)

	// apply registered plugins before starting
	if err := plugins.ApplyAll(app); err != nil {
		fmt.Fprintf(os.Stderr, "apply plugins: %v\n", err)
		os.Exit(1)
	}

	// You can still set views like other examples. For brevity we'll
	// just start the server.
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
