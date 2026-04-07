// Package app provides internal lifecycle management for the Flow framework.
//
// The Lifecycle type encapsulates the start/run/shutdown state machine for an
// HTTP server, decoupled from the public App type in pkg/flow. This separation
// makes it easy to test the lifecycle logic in isolation and keeps pkg/flow/app.go
// focused on wiring rather than mechanics.
//
// Typical usage inside pkg/flow:
//
//	lc := app.NewLifecycle(handler, addr, readTimeout, writeTimeout, idleTimeout)
//	if err := lc.Start(); err != nil { ... }
//	if err := lc.Run(ctx, shutdownTimeout, onShutdown...); err != nil { ... }
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/undiegomejia/flow/internal/server"
)

// ErrAlreadyRunning is returned when Start is called on a Lifecycle that is
// already running.
var ErrAlreadyRunning = errors.New("lifecycle: already running")

// state constants mirror server state transitions.
const (
	stateIdle    int32 = 0
	stateRunning int32 = 1
	stateStopped int32 = 2
)

// Logger is the minimal logging interface used by Lifecycle. It is satisfied
// by *log.Logger and any flow.Logger.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Lifecycle manages the start/run/shutdown state machine for an HTTP server.
// It is safe for use from multiple goroutines after construction, except that
// Start must not be called concurrently.
type Lifecycle struct {
	// handler is the http.Handler to serve. Set at construction time.
	handler http.Handler

	// Server configuration – set once at construction time.
	addr         string
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration

	// logger is used for lifecycle event messages.
	logger Logger

	// server is the underlying internal server; created in Start.
	server *server.Server

	// state: 0=idle, 1=running, 2=stopped.
	state int32
}

// NewLifecycle constructs a Lifecycle ready to be started. addr defaults to
// ":3000" if empty. If logger is nil the standard library logger is used.
func NewLifecycle(
	handler http.Handler,
	addr string,
	readTimeout, writeTimeout, idleTimeout time.Duration,
	logger Logger,
) *Lifecycle {
	if addr == "" {
		addr = ":3000"
	}
	if logger == nil {
		logger = log.New(os.Stdout, "[flow] ", log.LstdFlags)
	}
	return &Lifecycle{
		handler:      handler,
		addr:         addr,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		idleTimeout:  idleTimeout,
		logger:       logger,
	}
}

// Start begins listening in a background goroutine. It returns
// ErrAlreadyRunning if called while the server is already up.
func (lc *Lifecycle) Start() error {
	if !atomic.CompareAndSwapInt32(&lc.state, stateIdle, stateRunning) {
		return ErrAlreadyRunning
	}
	s := server.New(
		lc.handler,
		lc.addr,
		lc.readTimeout,
		lc.writeTimeout,
		lc.idleTimeout,
	)
	lc.server = s
	return s.Start()
}

// Run starts the server (calling Start internally) then blocks until either
// the context is canceled or SIGINT/SIGTERM is received. On exit it performs
// graceful shutdown and calls each onShutdown hook in order.
//
// shutdownTimeout controls how long to wait for in-flight requests to drain.
// A zero or negative value falls back to 10 seconds.
func (lc *Lifecycle) Run(ctx context.Context, shutdownTimeout time.Duration, onShutdown ...func(context.Context) error) error {
	if err := lc.Start(); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		lc.logger.Printf("context canceled, shutting down: %v", ctx.Err())
	case sig := <-sigCh:
		lc.logger.Printf("received signal %s, shutting down", sig)
	}

	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return lc.Shutdown(shutCtx, onShutdown...)
}

// Shutdown gracefully drains in-flight requests and then calls each
// onShutdown hook in order. It is safe to call multiple times; subsequent
// calls are no-ops.
func (lc *Lifecycle) Shutdown(ctx context.Context, onShutdown ...func(context.Context) error) error {
	if lc.server == nil {
		return nil
	}
	if !atomic.CompareAndSwapInt32(&lc.state, stateRunning, stateStopped) {
		if atomic.LoadInt32(&lc.state) == stateStopped {
			return nil
		}
	}

	if err := lc.server.Shutdown(ctx); err != nil {
		lc.logger.Printf("shutdown error: %v; attempting force close", err)
		if cerr := lc.server.Close(); cerr != nil {
			lc.logger.Printf("force close error: %v", cerr)
		}
		return fmt.Errorf("shutdown: %w", err)
	}

	// Run caller-supplied hooks (stop workers, flush tracer, etc.)
	var firstErr error
	for _, fn := range onShutdown {
		if fn == nil {
			continue
		}
		if err := fn(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Addr returns the configured listen address.
func (lc *Lifecycle) Addr() string { return lc.addr }

// IsRunning reports whether the server is currently listening.
func (lc *Lifecycle) IsRunning() bool {
	return atomic.LoadInt32(&lc.state) == stateRunning
}
