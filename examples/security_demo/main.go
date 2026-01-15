package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/undiegomejia/flow/pkg/flow"
)

func main() {
    app := flow.New("security-demo")

    // Enable secure defaults (HSTS, X-Frame-Options, etc.)
    _ = flow.WithSecureDefaults(app)

    app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hello secure world"))
    }))

    if err := app.Start(); err != nil {
        log.Fatalf("start: %v", err)
    }

    // run for a short while then shutdown
    time.Sleep(250 * time.Millisecond)
    _ = app.Shutdown(context.Background())
}
