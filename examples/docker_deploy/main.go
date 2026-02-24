package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	zapadapter "github.com/undiegomejia/flow/contrib/log/zap"
	"github.com/undiegomejia/flow/pkg/flow"
	"go.uber.org/zap"
)

func main() {
	// simple zap logger
	z, _ := zap.NewProduction()
	defer z.Sync()

	app := flow.New("docker-app",
		flow.WithLogger(zapadapter.NewZapAdapter(z)),
		flow.WithBoundedExecutor(4, 16),
		flow.WithDefaultMiddleware(),
	)

	// simple health endpoint registered on the App's router
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	app.SetRouter(mux)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      app.Handler(),
	}

	log.Printf("listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}

	// graceful shutdown (never reached in this tiny example)
	_ = srv.Shutdown(context.Background())
}
