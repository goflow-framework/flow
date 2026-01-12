// Package flow provides the application bootstrap for the Flow framework.
//
// This file implements an opinionated, minimal App type that wires together
// a router, middleware stack, HTTP server, and lifecycle utilities. It's
// intentionally small and testable: no global state, explicit options, and
// clear shutdown semantics.
//
// The App is responsible for:
// - holding configuration (address, timeouts, logger)
// - accepting a router (http.Handler) or using a default ServeMux
// - registering middleware in a deterministic order
// - starting and gracefully shutting down the HTTP server
//
// TODO: integrate with pkg/flow/router, controller, view and model packages
// when those modules are implemented. Add lifecycle hooks and health checks.
package flow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	orm "github.com/dministrator/flow/internal/orm"
	"github.com/uptrace/bun"
	"github.com/dministrator/flow/pkg/observability"
)

// Middleware is a function that wraps an http.Handler. Order matters: middleware
// registered earlier will be executed outer-most (first to receive requests).
type Middleware func(http.Handler) http.Handler

// Logger defines the subset of logging functionality Flow expects. Users can
// provide their own logger as long as it implements these methods.
type Logger interface {
	Printf(format string, v ...interface{})
}

// App encapsulates the running web application.
// It contains no global state and is safe for concurrent use after
// construction (except for calling Start multiple times).
type App struct {
	Name            string
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	logger Logger

	// router is the underlying http.Handler providing routing logic. If nil,
	// a default http.ServeMux is used.
	router http.Handler

	// Sessions holds the session manager used by the App. If nil, sessions
	// are disabled. Initialized with a default manager in New().
	Sessions *SessionManager

	// Views provides template rendering utilities for controllers and handlers.
	Views *ViewManager

	middleware []Middleware

	server *http.Server
	// db is the optional database connection attached to the App.
	db *sql.DB
	// bunAdapter holds an optional Bun adapter for ORM operations. If set,
	// App.Bun() returns the underlying *bun.DB for convenience.
	bunAdapter *orm.BunAdapter

	// tracerShutdown holds an optional shutdown function returned by a tracer
	// initializer (eg. observability.SetupStdoutTracer). If non-nil it will be
	// called during Shutdown to allow exporters to flush.
	tracerShutdown func(context.Context) error

	// state indicates whether the server is running: 0 = idle, 1 = running,
	// 2 = shutting down/stopped.
	state int32
}

// SetBun attaches a BunAdapter to the App and also sets the underlying *sql.DB
// so existing DB helpers continue to work.
func (a *App) SetBun(b *orm.BunAdapter) {
	if b == nil {
		a.bunAdapter = nil
		return
	}
	a.bunAdapter = b
	if b.SQLDB != nil {
		a.SetDB(b.SQLDB)
	}
}

// Bun returns the underlying *bun.DB if configured, or nil otherwise.
func (a *App) Bun() *bun.DB {
	if a == nil || a.bunAdapter == nil {
		return nil
	}
	return a.bunAdapter.DB
}

var (
	// ErrAppAlreadyRunning is returned when Start/Run is called on an already-running App.
	ErrAppAlreadyRunning = errors.New("app: already running")
)

// Option is a functional option for configuring an App at construction time.
type Option func(*App)

// WithLogger sets a custom logger. If not provided, the standard log.Logger is used.
func WithLogger(l Logger) Option {
	return func(a *App) { a.logger = l }
}

// WithBun attaches a BunAdapter to the App during construction.
func WithBun(b *orm.BunAdapter) Option {
	return func(a *App) { a.SetBun(b) }
}

// WithAddr sets the listen address (eg. ":3000").
func WithAddr(addr string) Option {
	return func(a *App) { a.Addr = addr }
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) Option {
	return func(a *App) { a.ShutdownTimeout = d }
}

// WithViewsDefaultLayout configures the default layout file (relative to the
// Views.TemplateDir) that will be parsed before rendering views.
func WithViewsDefaultLayout(layout string) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		if a.Views == nil {
			a.Views = NewViewManager("views")
		}
		a.Views.SetDefaultLayout(layout)
	}
}

// WithViewsDevMode toggles development mode for the ViewManager. When true
// templates are reparsed on each render and caching is disabled.
func WithViewsDevMode(dev bool) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		if a.Views == nil {
			a.Views = NewViewManager("views")
		}
		a.Views.SetDevMode(dev)
	}
}

// WithViewsFuncMap sets the template FuncMap on the ViewManager during App construction.
func WithViewsFuncMap(m template.FuncMap) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		if a.Views == nil {
			a.Views = NewViewManager("views")
		}
		a.Views.SetFuncMap(m)
	}
}

// WithLogging registers the built-in logging middleware using the App's logger.
func WithLogging() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(LoggingMiddleware(a.logger))
	}
}

// WithRequestID registers the request ID middleware. If headerName is empty
// the default header "X-Request-ID" is used.
func WithRequestID(headerName string) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(RequestIDMiddleware(headerName))
	}
}

// WithTimeout registers a per-request timeout middleware. A zero duration
// disables the timeout.
func WithTimeout(d time.Duration) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(TimeoutMiddleware(d))
	}
}

// WithMetrics registers a basic metrics middleware that sets X-Response-Time.
func WithMetrics() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(MetricsMiddleware())
	}
}

// WithDefaultMiddleware registers a sensible default middleware stack:
// Recovery, RequestID, Logging and Metrics.
func WithDefaultMiddleware() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(Recovery(a.logger))
		a.Use(RequestIDMiddleware(""))
		a.Use(LoggingMiddleware(a.logger))
		a.Use(MetricsMiddleware())
	}
}

// WithPrometheus registers the Prometheus instrumentation middleware (from
// pkg/observability). This should be used when the process also exposes a
// /metrics HTTP endpoint so the recorded metrics can be scraped.
func WithPrometheus() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.Use(observability.InstrumentHandler)
	}
}

// WithStdoutTracer initializes a simple stdout OpenTelemetry tracer during
// App construction. The returned shutdown function will be stored on the App
// and invoked during Shutdown so spans can be flushed. Errors during setup are
// logged to the App logger.
// WithStdoutTracer initializes a simple stdout OpenTelemetry tracer during
// App construction. The returned shutdown function will be stored on the App
// and invoked during Shutdown so spans can be flushed. Options allow tuning
// sampling and batch exporter behaviour.
func WithStdoutTracer(serviceName string, opts observability.StdoutTracerOptions) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		shutdown, err := observability.SetupStdoutTracer(serviceName, opts)
		if err != nil {
			if a.logger != nil {
				a.logger.Printf("failed to setup stdout tracer: %v", err)
			}
			return
		}
		a.tracerShutdown = shutdown
	}
}

// WithOTLPExporter configures an OTLP/gRPC exporter. endpoint is the OTLP
// collector address (host:port). If insecure is true TLS will be disabled
// (useful for local testing). headers may include auth headers like API keys.
func WithOTLPExporter(endpoint, serviceName string, insecure bool, headers map[string]string) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		shutdown, err := observability.SetupOTLPTracer(context.Background(), endpoint, insecure, headers, serviceName)
		if err != nil {
			if a.logger != nil {
				a.logger.Printf("failed to setup OTLP tracer: %v", err)
			}
			return
		}
		a.tracerShutdown = shutdown
	}
}

// New creates a configured App instance. It never starts network listeners.
func New(name string, opts ...Option) *App {
	// default logger
	stdLogger := log.New(os.Stdout, "[flow] ", log.LstdFlags)

	a := &App{
		Name:            name,
		Addr:            ":3000",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		logger:          stdLogger,
		router:          http.NewServeMux(),
		Views:           NewViewManager("views"),
		Sessions:        DefaultSessionManager(),
		middleware:      make([]Middleware, 0),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Use appends middleware to the middleware stack.
// Middlewares are applied in registration order with the first registered
// being the outer-most wrapper.
func (a *App) Use(m Middleware) {
	a.middleware = append(a.middleware, m)
}

// SetRouter replaces the App's router. If nil is provided the default
// ServeMux is used.
func (a *App) SetRouter(h http.Handler) {
	if h == nil {
		h = http.NewServeMux()
	}
	a.router = h
}

// Handler builds the final http.Handler by applying middleware to the router.
func (a *App) Handler() http.Handler {
	var h http.Handler = a.router
	// Apply middleware in reverse so the first registered is outer-most.
	for i := len(a.middleware) - 1; i >= 0; i-- {
		h = a.middleware[i](h)
	}
	return h
}

// Start starts the HTTP server in a background goroutine and returns immediately.
// It returns ErrAppAlreadyRunning if called while the server is already running.
func (a *App) Start() error {
	if !atomic.CompareAndSwapInt32(&a.state, 0, 1) {
		return ErrAppAlreadyRunning
	}

	srv := &http.Server{
		Addr:         a.Addr,
		Handler:      a.Handler(),
		ReadTimeout:  a.ReadTimeout,
		WriteTimeout: a.WriteTimeout,
		IdleTimeout:  a.IdleTimeout,
	}
	a.server = srv

	go func() {
		a.logger.Printf("starting %s on %s", a.Name, a.Addr)
		// http.ErrServerClosed is returned on normal shutdown and should not be logged as an error
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Printf("server error: %v", err)
		}
		// transition to stopped
		atomic.StoreInt32(&a.state, 2)
	}()

	return nil
}

// Run starts the server and blocks until a termination signal is received or
// the context is canceled. It performs a graceful shutdown with the configured
// ShutdownTimeout.
func (a *App) Run(ctx context.Context) error {
	if err := a.Start(); err != nil {
		return err
	}

	// listen for termination signals or context cancellation
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		a.logger.Printf("context canceled, shutting down: %v", ctx.Err())
	case sig := <-sigCh:
		a.logger.Printf("received signal %s, shutting down", sig)
	}

	// perform graceful shutdown with timeout
	t := a.ShutdownTimeout
	if t <= 0 {
		t = 10 * time.Second
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	return a.Shutdown(ctxShutdown)
}

// Shutdown gracefully stops the HTTP server. It is safe to call multiple times.
func (a *App) Shutdown(ctx context.Context) error {
	// if server is nil, nothing to do
	if a.server == nil {
		return nil
	}
	// only attempt shutdown once
	if !atomic.CompareAndSwapInt32(&a.state, 1, 2) {
		// if state is already 2 (shutting down/stopped), return nil
		if atomic.LoadInt32(&a.state) == 2 {
			return nil
		}
	}

	a.logger.Printf("shutting down %s", a.Name)
	if err := a.server.Shutdown(ctx); err != nil {
		// if forced close is required, attempt Close
		a.logger.Printf("shutdown error: %v; attempting force close", err)
		if cerr := a.server.Close(); cerr != nil {
			a.logger.Printf("force close error: %v", cerr)
		}
		return fmt.Errorf("shutdown: %w", err)
	}

	a.logger.Printf("shutdown complete")
	// allow tracer shutdown to flush/export spans if configured
	if a.tracerShutdown != nil {
		_ = a.tracerShutdown(ctx)
	}
	return nil
}

// ServeHTTP implements http.Handler so App can be used directly in tests.
// It dispatches to the composed handler (router + middleware).
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Handler().ServeHTTP(w, r)
}

// Default middleware helpers

// Recovery is a small middleware that recovers from panics and returns a
// 500 response. It logs the panic via the App logger.
func Recovery(logger Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Printf("panic: %v", rec)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// TODO: add more built-in middleware: logging, request ID, metrics, timeouts
