package main

import (
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"

	"github.com/dministrator/flow/pkg/plugins"
	"github.com/dministrator/flow/pkg/flow"
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

	// start pprof on :6060 (default mux)
	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
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
