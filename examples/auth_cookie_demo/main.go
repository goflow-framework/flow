package main

import (
	"context"
	"log"
	"net/http"
	"time"

	cookie "github.com/goflow-framework/flow/contrib/auth/cookie"
	"github.com/goflow-framework/flow/pkg/flow"
)

func main() {
	// simple demo app showing how to wire session + cookie auth middleware
	app := flow.New("auth-cookie-demo")

	// create a deterministic session manager for the demo (production apps
	// should provide a stable secret via configuration)
	sm := flow.DefaultSessionManager()

	// register session middleware first
	app.Use(sm.Middleware())

	// lookup function that resolves session uid -> user. In real apps this
	// would perform a DB lookup using a service registered on App.
	lookup := func(id interface{}) (interface{}, error) {
		if s, ok := id.(string); ok && s == "42" {
			return map[string]string{"id": s, "name": "demo-user"}, nil
		}
		return nil, nil
	}

	// register cookie auth middleware that reads "uid" from session
	app.Use(cookie.Middleware(sm, "uid", lookup))

	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := cookie.FromContext(r.Context()); ok && u != nil {
			w.Write([]byte("hello authenticated user"))
			return
		}
		w.Write([]byte("hello anonymous"))
	}))

	if err := app.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}
	// run briefly then shutdown
	time.Sleep(250 * time.Millisecond)
	_ = app.Shutdown(context.Background())
}
