package server

// Lightweight HTTP server wrapper that encapsulates starting and graceful
// shutdown logic used by pkg/flow.App. This package purposely avoids
// importing the top-level App type to prevent import cycles; it operates on
// net/http primitives and accepts callbacks for lifecycle hooks where needed.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// Server is a thin wrapper around http.Server that manages lifecycle and
// provides Start/Run/Shutdown primitives.
type Server struct {
	srv *http.Server
	// started indicates whether Start has been called: 0 = idle, 1 = started
	started int32
}

// New constructs a Server wrapping the provided handler and configuration.
func New(handler http.Handler, addr string, readTimeout, writeTimeout, idleTimeout time.Duration) *Server {
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	return &Server{srv: srv}
}

// Start launches the HTTP server in a background goroutine and returns
// immediately. If the server is already started, it returns an error.
func (s *Server) Start() error {
	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return fmt.Errorf("server: already started")
	}
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Note: logging is the caller's responsibility
			fmt.Printf("server error: %v\n", err)
		}
		atomic.StoreInt32(&s.started, 0)
	}()
	return nil
}

// Run starts the server (if not already started) and blocks until ctx is
// canceled or an OS termination signal is received. When stopping it will
// perform a graceful shutdown with the provided shutdownTimeout and call
// the onShutdown callback to allow the caller to clean up resources.
func (s *Server) Run(ctx context.Context, shutdownTimeout time.Duration, onShutdown func(context.Context) error) error {
	// start if necessary
	if atomic.LoadInt32(&s.started) == 0 {
		if err := s.Start(); err != nil {
			return err
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		// proceed to shutdown
	case <-sigCh:
		// proceed to shutdown
	}

	t := shutdownTimeout
	if t <= 0 {
		t = 10 * time.Second
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	if err := s.Shutdown(ctxShutdown); err != nil {
		return err
	}
	if onShutdown != nil {
		return onShutdown(ctxShutdown)
	}
	return nil
}

// Shutdown gracefully stops the underlying http.Server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// Close forcibly closes the underlying server (immediate stop).
func (s *Server) Close() error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Close()
}
