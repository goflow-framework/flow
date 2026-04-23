package main

import (
	"io"
	"log"
	"net/http"
	// pprof is intentionally available for local/debug runs of the tool.
	// The blank import registers the /debug/pprof handlers on the default mux.
	// This tool is explicitly a developer/debug utility so we allow the import.
	// #nosec G108
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/goflow-framework/flow/pkg/flow"
	"github.com/goflow-framework/flow/pkg/plugins"
)

func main() {
	// discard logs to avoid polluting benchmark
	logger := log.New(io.Discard, "pprof-server: ", log.LstdFlags)

	app := flow.New("pprof-app", flow.WithLogger(logger), flow.WithDefaultMiddleware())

	r := flow.NewRouter(app)

	// register multiple static routes to force iteration in router
	for i := 0; i < 200; i++ {
		p := "/static/route/" + string(rune(i))
		r.Get(p, func(c *flow.Context) { c.Status(200) })
	}

	// parameterized route we will hit from the client
	r.Get("/users/:id/profile", func(c *flow.Context) {
		// read param
		_ = c.Param("id")

		// configurable CPU work to amplify samples for pprof; default to 50k iterations.
		iters := 50000
		if v := os.Getenv("PPROF_WORK"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				iters = n
			}
		}
		// simple CPU-bound loop (no allocations) to create measurable CPU usage.
		sum := 0
		for i := 0; i < iters; i++ {
			sum += i * i
		}
		_ = sum

		c.Status(200)
	})

	app.SetRouter(r.Handler())

	// apply registered plugins before starting
	if err := plugins.ApplyAll(app); err != nil {
		log.Fatalf("apply plugins: %v", err)
	}

	// start pprof on :6060 (default mux). This is a debug-only tool and
	// intentionally exposes pprof on the loopback interface when developers
	// run it locally. We create a short-timeout server to avoid leaving an
	// unconstrained listener running indefinitely in tests.
	go func() {
		srv := &http.Server{
			Addr:         ":6060",
			Handler:      nil,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("pprof server error: %v", err)
			os.Exit(1)
		}
	}()

	// start app server on :8081
	if err := app.Start(); err != nil {
		log.Fatalf("failed to start app: %v", err)
	}

	select {}
}
