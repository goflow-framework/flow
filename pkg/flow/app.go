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
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/undiegomejia/flow/internal/config"
	orm "github.com/undiegomejia/flow/internal/orm"
	"github.com/undiegomejia/flow/internal/server"
	"github.com/undiegomejia/flow/pkg/assets"
	execpkg "github.com/undiegomejia/flow/pkg/exec"
	flowexec "github.com/undiegomejia/flow/pkg/flow/exec"
	"github.com/undiegomejia/flow/pkg/job"
	"github.com/undiegomejia/flow/pkg/observability"
	"github.com/uptrace/bun"
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

	server *server.Server
	// db is the optional database connection attached to the App.
	db *sql.DB
	// bunAdapter holds an optional Bun adapter for ORM operations. If set,
	// App.Bun() returns the underlying *bun.DB for convenience.
	bunAdapter *orm.BunAdapter

	// services is an application-scoped registry for sharing services between
	// components and plugins. Use App.RegisterService/GetService to access it.
	services *ServiceRegistry

	// tracerShutdown holds an optional shutdown function returned by a tracer
	// initializer (eg. observability.SetupStdoutTracer). If non-nil it will be
	// called during Shutdown to allow exporters to flush.
	tracerShutdown func(context.Context) error

	// state indicates whether the server is running: 0 = idle, 1 = running,
	// 2 = shutting down/stopped.
	state int32

	// executor is an optional application-level executor for background work.
	executor execpkg.Executor
	// executorShutdown is called during App.Shutdown if non-nil.
	executorShutdown func(context.Context) error

	// ctxPool holds an optional per-App context pool. If non-nil NewContext
	// will prefer this pool when creating request Contexts. Having a pool
	// per-App allows isolation and tuning per application instance.
	ctxPool *ContextPool

	// workerHandles tracks started job workers so we can stop them during
	// shutdown. Protected by workersMu.
	workers   []workerHandle
	workersMu sync.Mutex

	// plugins holds per-App plugin instances, initialized by the framework
	// or extensions. Access via App.Plugins()[name].
	plugins     map[string]Plugin
	pluginOrder []string
	pluginsMu   sync.RWMutex

	// plugin lifecycle control: a cancelable context used to run plugin Start
	// implementations and a WaitGroup to wait for them during shutdown.
	pluginCtx    context.Context
	pluginCancel context.CancelFunc
	pluginWg     sync.WaitGroup

	// Error handling: custom handler and verbosity flag. If no custom handler
	// is provided the framework will use DefaultErrorHandler (with verbose
	// controlled by verboseErrors).
	errorHandler  func(http.ResponseWriter, *http.Request, error)
	verboseErrors bool
	// redactionEnabled controls whether built-in logging middleware will
	// redact sensitive fields (see RedactMap/RedactedValue). Default is
	// true to preserve secure-by-default behavior; call WithRedaction(false)
	// when you want to manage redaction externally.
	redactionEnabled bool
	// redactionCfg holds per-App redaction configuration when set via
	// WithRedactionConfig. If zero-value the package defaults are used.
	redactionCfg RedactionConfig
}

type workerHandle struct {
	w    *job.Worker
	done chan struct{}
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

// Logger returns the App's configured logger. It never returns nil; if no
// logger was configured the default standard logger is returned and stored
// on the App for subsequent calls.
func (a *App) Logger() Logger {
	if a == nil {
		// best-effort fallback
		return log.New(os.Stdout, "[flow] ", log.LstdFlags)
	}
	if a.logger == nil {
		a.logger = log.New(os.Stdout, "[flow] ", log.LstdFlags)
	}
	return a.logger
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

// WithStructuredLogger sets a StructuredLogger on the App. Internally it
// wraps the StructuredLogger with a LoggerAdapter so the rest of the App
// (and middleware) can use the traditional Logger interface while middleware
// that prefers StructuredLogger will detect and use it.
func WithStructuredLogger(sl StructuredLogger) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		if sl == nil {
			return
		}
		a.logger = &LoggerAdapter{L: sl}
	}
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

// WithConfig applies a *config.Config to the App. It sets all transport-level
// fields (Addr, timeouts), wires the session manager secret from
// SecretKeyBase, and applies cookie security flags derived from the
// environment. Individual WithAddr / WithShutdownTimeout calls made after
// WithConfig will override the values set here.
//
// If cfg is nil the call is a no-op so it is safe to pass the result of
// config.Load() directly even when the config is optional.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) {
		if a == nil || cfg == nil {
			return
		}
		if cfg.Addr != "" {
			a.Addr = cfg.Addr
		}
		if cfg.ReadTimeout > 0 {
			a.ReadTimeout = cfg.ReadTimeout
		}
		if cfg.WriteTimeout > 0 {
			a.WriteTimeout = cfg.WriteTimeout
		}
		if cfg.IdleTimeout > 0 {
			a.IdleTimeout = cfg.IdleTimeout
		}
		if cfg.ShutdownTimeout > 0 {
			a.ShutdownTimeout = cfg.ShutdownTimeout
		}
		// Wire session secret and cookie security flags.
		if a.Sessions != nil {
			a.Sessions.secret = cfg.SecretKeyBytes()
			a.Sessions.CookieSecure = cfg.CookieSecure
			a.Sessions.CookieSameSite = cfg.CookieSameSite
		}
	}
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

// WithRedaction configures whether the framework's built-in logging
// middleware performs field redaction before emitting structured logs.
// By default redaction is enabled. Pass false to opt-out.
func WithRedaction(enabled bool) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.redactionEnabled = enabled
	}
}

// WithEmbeddedAssets wires embedded built assets into the App. It mounts the
// embedded asset filesystem under prefix (eg "/assets/") and registers an
// "asset" template function that resolves logical asset paths to their
// fingerprinted counterparts when a manifest is present at dist/manifest.json.
// If no manifest is found the function returns prefix+path.
func WithEmbeddedAssets(prefix string) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		if a.Views == nil {
			a.Views = NewViewManager("views")
		}

		// mount embedded assets (production)
		_ = a.MountAssets(prefix, assets.Assets(), "")

		// load manifest if present
		man, err := assets.LoadManifest("dist/manifest.json")

		// merge existing FuncMap preserving previous entries
		fm := template.FuncMap{}
		if a.Views.FuncMap != nil {
			for k, v := range a.Views.FuncMap {
				fm[k] = v
			}
		}

		if err == nil && man != nil {
			fm["asset"] = assets.AssetFuncFromManifest(man, prefix)
		} else {
			fm["asset"] = func(s string) string { return prefix + s }
		}
		a.Views.SetFuncMap(fm)
	}
}

// WithLogging registers the built-in logging middleware using the App's logger.
func WithLogging() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		// If the App has an explicit RedactionConfig use a middleware that
		// applies that config when emitting structured logs. Otherwise fall
		// back to the standard LoggingMiddleware which uses package defaults.
		if cfg := GetRedactionConfig(a); cfg != nil {
			a.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start := time.Now()
					fields := map[string]interface{}{
						"method": r.Method,
						"path":   r.URL.Path,
						"remote": r.RemoteAddr,
					}
					if sl, ok := a.logger.(StructuredLogger); ok {
						sl.Log("info", "request start", RedactMapWithConfig(cfg, fields))
					} else {
						a.logger.Printf("request start: %s %s", r.Method, r.URL.Path)
					}

					next.ServeHTTP(w, r)
					elapsed := time.Since(start)
					fields["elapsed"] = elapsed.String()
					if sl, ok := a.logger.(StructuredLogger); ok {
						sl.Log("info", "request complete", RedactMapWithConfig(cfg, fields))
					} else {
						a.logger.Printf("request complete: %s %s in %s", r.Method, r.URL.Path, elapsed)
					}
				})
			})
		} else {
			a.Use(LoggingMiddleware(a.logger))
		}
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

// WithDefaultMiddleware forward declaration removed; implementation is later in this file.

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

// WithExecutor sets a custom Executor on the App. The provided Executor's
// lifecycle is not managed by the App unless WithBoundedExecutor is used.
func WithExecutor(e execpkg.Executor) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.executor = e
	}
}

// WithBoundedExecutor creates a bounded executor with n workers and queueSize
// and wires it into the App. The App will call Shutdown on the executor
// during App.Shutdown.
func WithBoundedExecutor(n, queueSize int) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		be := flowexec.NewBoundedExecutor(n, queueSize)
		a.executor = be
		a.executorShutdown = be.Shutdown
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
		services:        NewServiceRegistry(), // Initialize services registry
		ctxPool:         NewContextPool(),
		plugins:         make(map[string]Plugin),
		// preserve secure-by-default redaction behavior for middleware/logging
		redactionEnabled: true,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// StartWorker starts a background job.Worker using the provided RedisQueue and
// handlers. If the App has an Executor configured it will be attached to the
// worker so job handlers run via the executor. StartWorker returns the
// started *job.Worker so callers may interact with it; the App will also
// manage shutdown of started workers.
func (a *App) StartWorker(q *job.RedisQueue, handlers map[string]job.Handler, opts job.WorkerOptions) *job.Worker {
	w := job.NewWorker(q, handlers, opts)
	if a.executor != nil {
		w.SetExecutor(a.executor)
	}
	// record handle and start worker in background
	done := make(chan struct{})
	a.workersMu.Lock()
	a.workers = append(a.workers, workerHandle{w: w, done: done})
	a.workersMu.Unlock()
	go func() {
		_ = w.Start(context.Background())
		close(done)
	}()
	return w
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

	// If the provided router exposes a URL(name, params) method, inject a
	// convenient `url_for` template helper into the ViewManager so templates
	// can generate named routes. We merge with any existing FuncMap to avoid
	// clobbering user-provided functions.
	type urler interface {
		URL(string, map[string]string) (string, error)
	}
	var u urler
	if h != nil {
		if uu, ok := h.(urler); ok {
			u = uu
		} else {
			// try a common public Router type which also implements URL
			if fr, ok := h.(*Router); ok {
				// Router.URL delegates to inner and implements urler
				u = fr
			}
		}
	}

	if u != nil && a != nil && a.Views != nil {
		fm := template.FuncMap{}
		if a.Views.FuncMap != nil {
			for k, v := range a.Views.FuncMap {
				fm[k] = v
			}
		}
		// url_for(name, "key", "value", "k2", value2...) or url_for(name, map[string]string{...})
		fm["url_for"] = func(name string, args ...interface{}) string {
			var params map[string]string
			if len(args) == 1 {
				if m, ok := args[0].(map[string]string); ok {
					params = m
				}
			}
			if params == nil && len(args) > 0 {
				// expect key/value pairs
				if len(args)%2 != 0 {
					return "#"
				}
				params = make(map[string]string, len(args)/2)
				for i := 0; i < len(args); i += 2 {
					k, ok := args[i].(string)
					if !ok {
						return "#"
					}
					params[k] = fmt.Sprint(args[i+1])
				}
			}
			ustr, err := u.URL(name, params)
			if err != nil {
				return "#"
			}
			return ustr
		}
		a.Views.SetFuncMap(fm)
	}
}

// Handler builds the final http.Handler by applying middleware to the router.
func (a *App) Handler() http.Handler {
	var h = a.router
	// Apply middleware in reverse so the first registered is outer-most.
	for i := len(a.middleware) - 1; i >= 0; i-- {
		h = a.middleware[i](h)
	}
	// Always wrap with recovery middleware that converts panics into errors and
	// delegates to the configured error handler.
	return a.recoveryMiddleware(h)
}

// recoveryMiddleware returns a middleware that recovers from panics and
// delegates error rendering to App.errorHandler or DefaultErrorHandler.
func (a *App) recoveryMiddleware(next http.Handler) http.Handler {
	if a == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				switch v := rec.(type) {
				case error:
					err = v
				default:
					err = fmt.Errorf("panic: %v", v)
				}
				if a.errorHandler != nil {
					a.errorHandler(w, r, err)
					return
				}
				DefaultErrorHandler(w, r, err, a.verboseErrors)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WithErrorHandler sets a custom error handler to render errors for requests.
func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(a *App) {
		if a == nil {
			return
		}
		a.errorHandler = fn
	}
}

// WithVerboseErrors toggles whether the default error handler exposes internal
// error messages in HTTP responses (only enable in non-production/dev).
func WithVerboseErrors(v bool) Option {
	return func(a *App) {
		if a != nil {
			a.verboseErrors = v
		}
	}
}

// RegisterPlugin registers a plugin with this App instance. It validates the
// plugin version, reserves the plugin name to avoid races, runs Init and
// Mount with the provided App, and registers any returned middleware on the
// App. Registration is isolated to this App instance and does not affect the
// package-level registry in pkg/plugins.
func (a *App) RegisterPlugin(p Plugin) error {
	if a == nil {
		return fmt.Errorf("app: nil")
	}
	if p == nil {
		return fmt.Errorf("plugin: nil")
	}
	if err := ValidatePluginVersion(p.Version()); err != nil {
		return err
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin: empty name")
	}

	// Reserve the name to prevent concurrent registrations racing.
	a.pluginsMu.Lock()
	if _, ok := a.plugins[name]; ok {
		a.pluginsMu.Unlock()
		return fmt.Errorf("plugin already registered: %s", name)
	}
	// store nil to mark reserved
	a.plugins[name] = nil
	a.pluginOrder = append(a.pluginOrder, name)
	a.pluginsMu.Unlock()

	// If Init/Mount fail we must cleanup the reservation.
	cleanup := func() {
		a.pluginsMu.Lock()
		delete(a.plugins, name)
		// remove from order
		for i, n := range a.pluginOrder {
			if n == name {
				a.pluginOrder = append(a.pluginOrder[:i], a.pluginOrder[i+1:]...)
				break
			}
		}
		a.pluginsMu.Unlock()
	}

	if err := p.Init(a); err != nil {
		cleanup()
		return err
	}
	if err := p.Mount(a); err != nil {
		cleanup()
		return err
	}
	if mws := p.Middlewares(); mws != nil {
		for _, mw := range mws {
			a.Use(mw)
		}
	}

	// finally, store the actual plugin instance
	a.pluginsMu.Lock()
	a.plugins[name] = p
	a.pluginsMu.Unlock()
	return nil
}

// RegisterService registers a named service in the App's ServiceRegistry.
// It returns an error if the service name is invalid or already registered.
func (a *App) RegisterService(name string, svc interface{}) error {
	if a == nil || a.services == nil {
		return fmt.Errorf("app: services not initialized")
	}
	return a.services.Register(name, svc)
}

// GetService looks up a previously registered service by name.
func (a *App) GetService(name string) (interface{}, bool) {
	if a == nil || a.services == nil {
		return nil, false
	}
	return a.services.Get(name)
}

// RegisterServiceTyped is a generic convenience wrapper that registers a
// typed service under name. It preserves the existing behavior but provides
// nicer ergonomics for callers using Go generics.
// (Typed service helpers are provided as package-level generic functions in service_registry.go)

// ListPlugins returns the registered plugin names for this App in
// registration order.
func (a *App) ListPlugins() []string {
	if a == nil {
		return nil
	}
	a.pluginsMu.RLock()
	defer a.pluginsMu.RUnlock()
	out := make([]string, len(a.pluginOrder))
	copy(out, a.pluginOrder)
	return out
}

// shutdownPlugins invokes Stop(ctx) on registered plugins in reverse
// registration order and aggregates errors similarly to pkg/plugins.ShutdownAll.
func (a *App) shutdownPlugins(ctx context.Context) error {
	if a == nil {
		return nil
	}
	a.pluginsMu.RLock()
	names := make([]string, len(a.pluginOrder))
	copy(names, a.pluginOrder)
	a.pluginsMu.RUnlock()

	var errs []string
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		a.pluginsMu.RLock()
		p := a.plugins[name]
		a.pluginsMu.RUnlock()
		if p == nil {
			continue
		}
		if err := p.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("plugin shutdown error: %s", errs[0])
	default:
		return fmt.Errorf("plugin shutdown errors: %s", strings.Join(errs, "; "))
	}
}

// startPlugins invokes Start(ctx) for each registered plugin in registration
// order in its own goroutine. Errors returned by Start are logged. The provided
// ctx is canceled by App.Shutdown (via pluginCancel) to signal plugin goroutines
// to exit.
func (a *App) startPlugins(ctx context.Context) {
	if a == nil {
		return
	}
	// copy names to avoid holding lock
	a.pluginsMu.RLock()
	names := make([]string, len(a.pluginOrder))
	copy(names, a.pluginOrder)
	a.pluginsMu.RUnlock()

	for _, name := range names {
		a.pluginsMu.RLock()
		p := a.plugins[name]
		a.pluginsMu.RUnlock()
		if p == nil {
			continue
		}
		// start each plugin in its own goroutine and track via WaitGroup
		a.pluginWg.Add(1)
		go func(p Plugin, n string) {
			defer a.pluginWg.Done()
			if err := p.Start(ctx); err != nil {
				a.Logger().Printf("plugin %s start error: %v", n, err)
			}
		}(p, name)
	}
}

// Start starts the HTTP server in a background goroutine and returns immediately.
// It returns ErrAppAlreadyRunning if called while the server is already running.
func (a *App) Start() error {
	if !atomic.CompareAndSwapInt32(&a.state, 0, 1) {
		return ErrAppAlreadyRunning
	}

	// construct internal server wrapper and start it
	s := server.New(a.Handler(), a.Addr, a.ReadTimeout, a.WriteTimeout, a.IdleTimeout)
	a.server = s

	// create a plugin context for Start lifecycle. It will be canceled in Shutdown.
	a.pluginCtx, a.pluginCancel = context.WithCancel(context.Background())

	// start the HTTP server
	if err := s.Start(); err != nil {
		return err
	}

	// Start plugin goroutines (they should return when pluginCtx is canceled)
	a.startPlugins(a.pluginCtx)

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

	// Cancel plugin Start contexts so plugin goroutines can exit.
	if a.pluginCancel != nil {
		a.pluginCancel()
		// wait for plugin goroutines to finish, but respect Shutdown ctx
		done := make(chan struct{})
		go func() {
			a.pluginWg.Wait()
			close(done)
		}()
		select {
		case <-done:
			// plugins stopped
		case <-ctx.Done():
			return ctx.Err()
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

	// stop any started job workers and wait for them to exit
	if len(a.workers) > 0 {
		a.workersMu.Lock()
		handles := make([]workerHandle, len(a.workers))
		copy(handles, a.workers)
		a.workersMu.Unlock()
		for _, h := range handles {
			// signal worker to stop
			h.w.Stop()
		}
		// wait for each worker's done channel or context cancellation
		for _, h := range handles {
			select {
			case <-h.done:
				// worker finished
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// shutdown executor if App created/owns one
	if a.executorShutdown != nil {
		_ = a.executorShutdown(ctx)
	}

	// allow tracer shutdown to flush/export spans if configured
	if a.tracerShutdown != nil {
		_ = a.tracerShutdown(ctx)
	}

	// shutdown plugins (calls Stop(ctx) in reverse order)
	_ = a.shutdownPlugins(ctx)

	return nil
}

// ServeHTTP implements http.Handler so App can be used directly in tests.
// It dispatches to the composed handler (router + middleware).
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Handler().ServeHTTP(w, r)
}

// MountAssets mounts a handler on the given prefix to serve static assets.
// If devProxy is non-empty it will proxy requests to the provided URL (eg
// http://localhost:8000) which is useful when running a local bundler/dev
// server. When devProxy is empty the provided fs is served via http.FileServer.
//
// prefix should include leading and trailing slash, e.g. "/assets/".
func (a *App) MountAssets(prefix string, fs http.FileSystem, devProxy string) error {
	if a == nil {
		return fmt.Errorf("app is nil")
	}
	if prefix == "" {
		prefix = "/assets/"
	}

	var h http.Handler
	if devProxy != "" {
		u, err := url.Parse(devProxy)
		if err != nil {
			return fmt.Errorf("invalid devProxy url: %w", err)
		}
		proxy := httputil.NewSingleHostReverseProxy(u)
		// strip prefix before proxying
		h = http.StripPrefix(prefix, proxy)
	} else {
		h = http.StripPrefix(prefix, http.FileServer(fs))
	}

	// If underlying router is a *http.ServeMux we can register the handler
	// directly. Otherwise wrap the existing router into a new ServeMux so
	// the mounted path takes precedence and the previous router handles the
	// rest.
	if mux, ok := a.router.(*http.ServeMux); ok {
		mux.Handle(prefix, h)
	} else {
		newMux := http.NewServeMux()
		newMux.Handle(prefix, h)
		// fallback to previous router for other paths
		newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			a.router.ServeHTTP(w, r)
		})
		a.router = newMux
	}
	return nil
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

// This function is opinionated: it wires a conservative secure-by-default
// stack appropriate for typical web apps. To opt-out of specific pieces,
// construct your App with the smaller building blocks instead (for example
// use WithRequestID, WithLogging, WithMetrics and omit WithDefaultMiddleware),
// or register only the middleware you need manually via App.Use.
func WithDefaultMiddleware() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		// recovery outer-most
		a.Use(Recovery(a.logger))
		// request id for tracing
		a.Use(RequestIDMiddleware(""))
		// secure headers (HSTS, X-Frame-Options, etc.)
		a.Use(SecureHeaders())
		// protect forms and unsafe methods with a per-session CSRF token
		a.Use(CSRFMiddleware())
		// lightweight, in-process rate limiting per client IP
		a.Use(RateLimitMiddleware(DefaultRateLimitRPS, DefaultRateLimitBurst))
		// logging and basic metrics (response time, status codes)
		if a.redactionEnabled {
			a.Use(LoggingMiddlewareWithRedaction(a.logger))
		} else {
			a.Use(LoggingMiddleware(a.logger))
		}
		a.Use(MetricsMiddleware())
	}
}

// WithLoggingLegacy (internal) registers the built-in logging middleware using the App's logger.
// Kept for compatibility during migration; prefer the later WithLogging which uses structured logging.
func WithLoggingLegacy() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		// Use a closure that captures the App so we can consult per-App
		// redaction configuration via GetRedactionConfig when emitting
		// structured log entries.
		a.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				fields := map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
					"remote": r.RemoteAddr,
				}
				if sl, ok := a.logger.(StructuredLogger); ok {
					sl.Log("info", "request start", RedactMapWithConfig(GetRedactionConfig(a), fields))
				} else {
					a.logger.Printf("request start: %s %s", r.Method, r.URL.Path)
				}
				next.ServeHTTP(w, r)
				elapsed := time.Since(start)
				fields["elapsed"] = elapsed.String()
				if sl, ok := a.logger.(StructuredLogger); ok {
					sl.Log("info", "request complete", RedactMapWithConfig(GetRedactionConfig(a), fields))
				} else {
					a.logger.Printf("request complete: %s %s in %s", r.Method, r.URL.Path, elapsed)
				}
			})
		})
	}
}

// WithSecureCookieDefaults is an App construction-time Option that enables
// conservative session cookie defaults and installs the SessionCookieHardening
// middleware so Set-Cookie headers produced by handlers are hardened when
// missing attributes. This is provided as an opt-in constructor option to
// allow operators to enable secure defaults centrally without changing user
// landed code.
func WithSecureCookieDefaults() Option {
	return func(a *App) {
		if a == nil {
			return
		}
		// ensure a session manager exists so we can apply defaults
		if a.Sessions == nil {
			a.Sessions = DefaultSessionManager()
		}
		a.Sessions.ApplySecureCookieDefaults()
		// register cookie hardening middleware so existing Set-Cookie headers
		// get conservative attributes (Secure; SameSite=Lax) when missing.
		a.Use(SessionCookieHardening())
	}
}

// TODO: add more built-in middleware: logging, request ID, metrics, timeouts
