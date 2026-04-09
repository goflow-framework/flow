package flow

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Health endpoint paths registered by EnableHealthz / WithHealthz.
const (
	HealthzPath = "/healthz"
	LivezPath   = "/livez"
)

// healthResponse is the JSON body returned by the health endpoints.
type healthResponse struct {
	// Status is "ok" or "unavailable".
	Status string `json:"status"`
	// App is the App.Name field, included for multi-app deployments.
	App string `json:"app,omitempty"`
	// Uptime is the duration since the App's EnableHealthz call, truncated to seconds.
	Uptime string `json:"uptime,omitempty"`
}

// WithHealthz is an option that mounts the /healthz (readiness) and /livez
// (liveness) endpoints on the App's router at construction time.
//
//   - GET /healthz — 200 while the server is in the running state, 503 otherwise.
//   - GET /livez   — always 200; used by liveness probes to detect deadlocked processes.
//
// The endpoints bypass all user-registered middleware (body-limit, auth,
// rate-limiting, etc.) so orchestrators can always reach them.
func WithHealthz() Option {
	return func(a *App) {
		a.EnableHealthz()
	}
}

// EnableHealthz mounts the built-in /healthz and /livez endpoints on the
// App's router. It is safe to call after New() and before Start(). Calling it
// more than once is a no-op (the second call is silently ignored so ServeMux
// does not panic on duplicate pattern registration).
func (a *App) EnableHealthz() {
	if a == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&a.healthzRegistered, 0, 1) {
		return
	}

	startedAt := time.Now()

	// readinessHandler responds 200 only while the App is in state==1 (running).
	// During startup (state 0) and after shutdown (state 2) it returns 503 so
	// load-balancers stop sending new traffic.
	readinessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if atomic.LoadInt32(&a.state) == 1 {
			w.WriteHeader(http.StatusOK)
			writeHealthJSON(w, healthResponse{
				Status: "ok",
				App:    a.Name,
				Uptime: time.Since(startedAt).Truncate(time.Second).String(),
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			writeHealthJSON(w, healthResponse{Status: "unavailable", App: a.Name})
		}
	})

	// livenessHandler always returns 200: as long as the process can handle HTTP
	// the container is alive. Kubernetes uses this to decide whether to restart.
	livenessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeHealthJSON(w, healthResponse{Status: "ok", App: a.Name})
	})

	a.mountHealthHandler(HealthzPath, readinessHandler)
	a.mountHealthHandler(LivezPath, livenessHandler)
}

// mountHealthHandler registers h on the internal mux at the given exact path,
// bypassing user-registered middleware. It uses the same ServeMux-wrapping
// strategy as MountAssets.
func (a *App) mountHealthHandler(path string, h http.Handler) {
	if mux, ok := a.router.(*http.ServeMux); ok {
		mux.Handle(path, h)
	} else {
		newMux := http.NewServeMux()
		newMux.Handle(path, h)
		prev := a.router
		newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			prev.ServeHTTP(w, r)
		})
		a.router = newMux
	}
}

// writeHealthJSON encodes v as JSON into w. The response header is already
// written at this point so encoding errors are silently discarded.
func writeHealthJSON(w http.ResponseWriter, v healthResponse) {
	_ = json.NewEncoder(w).Encode(v)
}
