package plugins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
)

// middlewarePlugin returns a middleware that injects a header to responses.
type middlewarePlugin struct{}

func (p *middlewarePlugin) Name() string            { return "mw-test" }
func (p *middlewarePlugin) Version() string         { return "v0.0.0" }
func (p *middlewarePlugin) Init(a *flow.App) error  { return nil }
func (p *middlewarePlugin) Mount(a *flow.App) error { return nil }
func (p *middlewarePlugin) Middlewares() []flow.Middleware {
	return []flow.Middleware{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// set header before calling next
				w.Header().Set("X-Plugin-Middleware", "1")
				next.ServeHTTP(w, r)
			})
		},
	}
}
func (p *middlewarePlugin) Start(ctx context.Context) error { return nil }
func (p *middlewarePlugin) Stop(ctx context.Context) error  { return nil }

// TestMiddlewareIsApplied verifies that middleware returned by plugins is
// registered on the App and runs for incoming requests.
func TestMiddlewareIsApplied(t *testing.T) {
	Reset()

	if err := Register(&middlewarePlugin{}); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	app := flow.New("mw-test")

	// set a simple handler that writes a body
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	app.SetRouter(mux)

	if err := ApplyAll(app); err != nil {
		t.Fatalf("apply plugins: %v", err)
	}

	// exercise the handler via httptest
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Plugin-Middleware"); got != "1" {
		t.Fatalf("expected middleware header, got %q", got)
	}
}
