package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/undiegomejia/flow/pkg/flow"
)

func main() {
	// Create a JSON structured logger that will receive structured entries.
	jl := flow.NewJSONLogger(os.Stdout)

	// Build the app with a per-App redaction configuration. WithRedactionConfig
	// instructs the logging middleware to redact keys listed below and also to
	// treat strings longer than MaxLen as sensitive.
	app := flow.New("redaction-demo",
		flow.WithStructuredLogger(jl),
		flow.WithRedactionConfig([]string{"api_key", "session"}, 8),
		flow.WithLogging(),
	)

	// Simple handler — the logging middleware registered by WithLogging will
	// emit structured logs for requests and apply the per-App redaction rules.
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	// Start the app in the background and shut it down shortly after.
	go func() {
		if err := app.Start(); err != nil {
			log.Fatalf("start: %v", err)
		}
	}()

	// Let the server run briefly so you can exercise it (eg. curl localhost:3000)
	time.Sleep(250 * time.Millisecond)

	_ = app.Shutdown(nil)
}
