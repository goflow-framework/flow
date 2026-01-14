package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/undiegomejia/flow/pkg/flow"
	"github.com/undiegomejia/flow/pkg/plugins"
)

// middlewarePlugin demonstrates returning middleware from a plugin.
type middlewarePlugin struct{}

func (p *middlewarePlugin) Name() string            { return "example-mw" }
func (p *middlewarePlugin) Version() string         { return "v0.0.0" }
func (p *middlewarePlugin) Init(a *flow.App) error  { return nil }
func (p *middlewarePlugin) Mount(a *flow.App) error { return nil }
func (p *middlewarePlugin) Middlewares() []flow.Middleware {
	return []flow.Middleware{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// inject header and continue
				w.Header().Set("X-From-Middleware", "1")
				next.ServeHTTP(w, r)
			})
		},
	}
}
func (p *middlewarePlugin) Start(ctx context.Context) error { return nil }
func (p *middlewarePlugin) Stop(ctx context.Context) error  { return nil }

func main() {
	// Use runtime registration in this example
	if err := plugins.Register(&middlewarePlugin{}); err != nil {
		log.Fatalf("register plugin: %v", err)
	}

	app := flow.New("middleware-example")

	// set a test handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	app.SetRouter(mux)

	if err := plugins.ApplyAll(app); err != nil {
		log.Fatalf("apply plugins: %v", err)
	}

	// start an httptest server using the app's composed handler so we don't need
	// to pick a concrete port (safe for CI and local runs).
	ts := httptest.NewServer(app.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		log.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d body=%s X-From-Middleware=%s\n", resp.StatusCode, string(b), resp.Header.Get("X-From-Middleware"))

	_ = app.Shutdown(context.Background())
}
